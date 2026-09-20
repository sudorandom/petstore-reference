package test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/pet"
	"github.com/example/pets/internal/telemetry"
	"github.com/example/pets/internal/testutil"
	"github.com/sudorandom/protojsonx/protojsonxconnect"
)

type PetServiceIntegrationTestSuite struct {
	suite.Suite

	ctx              context.Context
	cancel           context.CancelFunc
	testDB           *testutil.TestDB
	pool             *pgxpool.Pool
	server           *httptest.Server
	client           petv1connect.PetServiceClient
	createdPetID     string
	uploadedPhotoID  string
	uploadedPhotoURL string
}

func (s *PetServiceIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	testDB, err := testutil.StartTestDB(s.ctx)
	if err != nil {
		s.T().Skipf("Skipping integration test: Docker/Testcontainers not available: %v", err)
		return
	}
	s.testDB = testDB
	s.pool = testDB.Pool

	queries := db.New(s.pool)
	petHandler := pet.NewHandler(s.pool)

	otelInterceptor, err := telemetry.NewConnectInterceptor()
	s.Require().NoError(err, "failed to create otel interceptor")

	valInterceptor := validate.NewInterceptor()
	authCfg := auth.Config{
		Enabled:      true,
		StaticTokens: []string{"test-token"},
	}
	authInterceptor := auth.NewInterceptor(authCfg)

	path, handler := petv1connect.NewPetServiceHandler(
		petHandler,
		connect.WithCodec(&protojsonxconnect.Codec{}),
		connect.WithInterceptors(otelInterceptor, valInterceptor, authInterceptor),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("GET /photos/{id}", auth.Middleware(authCfg)(pet.NewPhotoHandler(queries)))
	s.server = httptest.NewServer(mux)

	s.client = petv1connect.NewPetServiceClient(
		s.server.Client(),
		s.server.URL,
		connect.WithCodec(&protojsonxconnect.Codec{}),
	)
}

func (s *PetServiceIntegrationTestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.testDB != nil {
		s.testDB.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *PetServiceIntegrationTestSuite) SetupTest() {
	if s.testDB != nil {
		s.Require().NoError(s.testDB.TruncateTables(s.ctx))
	}
}

func (s *PetServiceIntegrationTestSuite) TestPetLifecycle() {
	s.Run("CreatePet", func() {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:               "Milo",
			Species:            "Dog",
			BirthDate:          "2023-05-10",
			BirthDateEstimated: false,
			Status:             petv1.PetStatus_PET_STATUS_AVAILABLE,
			Tags:               []string{"friendly", "playful"},
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.CreatePet(s.ctx, req)
		s.Require().NoError(err, "CreatePet failed")
		s.Require().NotNil(resp.Msg)
		s.Require().NotNil(resp.Msg.Pet)

		s.Equal("Milo", resp.Msg.Pet.Name)
		s.Equal("2023-05-10", resp.Msg.Pet.BirthDate)
		s.False(resp.Msg.Pet.BirthDateEstimated)
		s.NotEmpty(resp.Msg.Pet.Id)
		s.NotEmpty(resp.Msg.Pet.CreatedBy)
		s.NotEmpty(resp.Msg.Pet.ModifiedBy)
		s.NotNil(resp.Msg.Pet.CreatedAt)
		s.NotNil(resp.Msg.Pet.ModifiedAt)

		s.createdPetID = resp.Msg.Pet.Id
	})

	s.Run("GetPet", func() {
		s.Require().NotEmpty(s.createdPetID, "skipping GetPet because CreatePet did not succeed")

		req := connect.NewRequest(&petv1.GetPetRequest{Id: s.createdPetID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.GetPet(s.ctx, req)
		s.Require().NoError(err, "GetPet failed")
		s.Require().NotNil(resp.Msg)
		s.Require().NotNil(resp.Msg.Pet)
		s.Equal(s.createdPetID, resp.Msg.Pet.Id)
	})

	s.Run("ListPets", func() {
		req := connect.NewRequest(&petv1.ListPetsRequest{
			Species:  "Dog",
			PageSize: 10,
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.ListPets(s.ctx, req)
		s.Require().NoError(err, "ListPets failed")
		s.Require().NotNil(resp.Msg)
		s.NotEmpty(resp.Msg.Pets)
	})

	s.Run("UpdatePet", func() {
		s.Require().NotEmpty(s.createdPetID, "skipping UpdatePet")

		req := connect.NewRequest(&petv1.UpdatePetRequest{
			Id:                 s.createdPetID,
			Name:               "Milo The Great",
			Species:            "Dog",
			BirthDate:          "2022-04-12",
			BirthDateEstimated: true,
			Status:             petv1.PetStatus_PET_STATUS_ADOPTED,
			Tags:               []string{"adopted", "happy"},
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.UpdatePet(s.ctx, req)
		s.Require().NoError(err, "UpdatePet failed")
		s.Require().NotNil(resp.Msg)
		s.Require().NotNil(resp.Msg.Pet)
		s.Equal("Milo The Great", resp.Msg.Pet.Name)
		s.Equal("2022-04-12", resp.Msg.Pet.BirthDate)
		s.True(resp.Msg.Pet.BirthDateEstimated)
		s.Equal(petv1.PetStatus_PET_STATUS_ADOPTED, resp.Msg.Pet.Status)
	})

	s.Run("UploadPetPhoto", func() {
		s.Require().NotEmpty(s.createdPetID, "skipping UploadPetPhoto")

		dummyPhoto := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46} // JPEG header bytes

		req := connect.NewRequest(&petv1.UploadPetPhotoRequest{
			PetId:    s.createdPetID,
			Data:     dummyPhoto,
			MimeType: "image/jpeg",
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.UploadPetPhoto(s.ctx, req)
		s.Require().NoError(err, "UploadPetPhoto failed")
		s.Require().NotNil(resp.Msg)
		s.NotEmpty(resp.Msg.PhotoId)

		s.uploadedPhotoID = resp.Msg.PhotoId
		s.uploadedPhotoURL = resp.Msg.PhotoUrl

		expectedPrefix := "/photos/" + s.uploadedPhotoID
		s.Equal(expectedPrefix, resp.Msg.PhotoUrl)
		s.Require().Len(resp.Msg.Pet.Photos, 1)
		s.Equal(s.uploadedPhotoID, resp.Msg.Pet.Photos[0].Id)
		s.Equal(s.uploadedPhotoURL, resp.Msg.Pet.Photos[0].Url)
	})

	s.Run("ServePetPhotoHTTP", func() {
		s.Require().NotEmpty(s.uploadedPhotoURL, "skipping ServePetPhotoHTTP")

		photoURL := s.server.URL + s.uploadedPhotoURL

		// GET request
		httpReq, err := http.NewRequestWithContext(s.ctx, http.MethodGet, photoURL, nil)
		s.Require().NoError(err)
		httpReq.Header.Set("Authorization", "Bearer test-token")
		httpResp, err := s.server.Client().Do(httpReq)
		s.Require().NoError(err, "HTTP GET photo failed")
		defer httpResp.Body.Close()

		s.Equal(http.StatusOK, httpResp.StatusCode)
		s.Equal("image/jpeg", httpResp.Header.Get("Content-Type"))
		s.Equal("private, max-age=86400", httpResp.Header.Get("Cache-Control"))

		body, err := io.ReadAll(httpResp.Body)
		s.Require().NoError(err, "failed to read response body")
		s.Len(body, 10)

		// HEAD request - handled automatically by http.ServeMux when mounted with GET
		headReq, err := http.NewRequestWithContext(s.ctx, http.MethodHead, photoURL, nil)
		s.Require().NoError(err)
		headReq.Header.Set("Authorization", "Bearer test-token")
		headResp, err := s.server.Client().Do(headReq)
		s.Require().NoError(err, "HTTP HEAD photo failed")
		defer headResp.Body.Close()

		s.Equal(http.StatusOK, headResp.StatusCode)
		s.Equal("image/jpeg", headResp.Header.Get("Content-Type"))

		// POST request - rejected with 405 Method Not Allowed
		postResp, err := s.server.Client().Post(photoURL, "text/plain", nil)
		s.Require().NoError(err, "HTTP POST photo failed")
		defer postResp.Body.Close()

		s.Equal(http.StatusMethodNotAllowed, postResp.StatusCode)
	})

	s.Run("DeletePetPhoto", func() {
		s.Require().NotEmpty(s.uploadedPhotoID, "skipping DeletePetPhoto")

		req := connect.NewRequest(&petv1.DeletePetPhotoRequest{PhotoId: s.uploadedPhotoID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.DeletePetPhoto(s.ctx, req)
		s.Require().NoError(err, "DeletePetPhoto failed")
		s.Require().NotNil(resp.Msg)
		s.True(resp.Msg.Success)

		// Verify photo is gone
		httpReq, reqErr := http.NewRequestWithContext(s.ctx, http.MethodGet, s.server.URL+s.uploadedPhotoURL, nil)
		s.Require().NoError(reqErr)
		httpReq.Header.Set("Authorization", "Bearer test-token")
		httpResp, reqErr := s.server.Client().Do(httpReq)
		s.Require().NoError(reqErr)
		defer httpResp.Body.Close()
		s.Equal(http.StatusNotFound, httpResp.StatusCode)
	})

	s.Run("DeletePet", func() {
		s.Require().NotEmpty(s.createdPetID, "skipping DeletePet")

		req := connect.NewRequest(&petv1.DeletePetRequest{Id: s.createdPetID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.DeletePet(s.ctx, req)
		s.Require().NoError(err, "DeletePet failed")
		s.Require().NotNil(resp.Msg)
		s.True(resp.Msg.Success)

		// Verify pet is gone
		getReq := connect.NewRequest(&petv1.GetPetRequest{Id: s.createdPetID})
		getReq.Header().Set("Authorization", "Bearer test-token")
		_, err = s.client.GetPet(s.ctx, getReq)
		s.Require().Error(err, "expected pet to be not found")
		s.Equal(connect.CodeNotFound, connect.CodeOf(err))
	})
}

func (s *PetServiceIntegrationTestSuite) TestUnauthenticatedCallRejection() {
	req := connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Shadow",
		Species:   "Cat",
		BirthDate: "2023-01-01",
	})
	// No Authorization header
	_, err := s.client.CreatePet(s.ctx, req)
	s.Require().Error(err)
	s.Equal(connect.CodeUnauthenticated, connect.CodeOf(err))
}

func (s *PetServiceIntegrationTestSuite) TestFilteringAndPagination() {
	// Database is automatically truncated before this test starts
	petsToCreate := []struct {
		name    string
		species string
		status  petv1.PetStatus
	}{
		{"Bella", "Dog", petv1.PetStatus_PET_STATUS_AVAILABLE},
		{"Max", "Dog", petv1.PetStatus_PET_STATUS_ADOPTED},
		{"Luna", "Cat", petv1.PetStatus_PET_STATUS_AVAILABLE},
		{"Charlie", "Cat", petv1.PetStatus_PET_STATUS_PENDING},
		{"Lucy", "Bird", petv1.PetStatus_PET_STATUS_AVAILABLE},
	}

	for _, p := range petsToCreate {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:      p.name,
			Species:   p.species,
			Status:    p.status,
			BirthDate: "2023-01-01",
		})
		req.Header().Set("Authorization", "Bearer test-token")
		_, err := s.client.CreatePet(s.ctx, req)
		s.Require().NoError(err)
	}

	// Filter by species "Dog"
	dogReq := connect.NewRequest(&petv1.ListPetsRequest{
		Species: "Dog",
	})
	dogReq.Header().Set("Authorization", "Bearer test-token")
	dogResp, err := s.client.ListPets(s.ctx, dogReq)
	s.Require().NoError(err)
	s.Equal(int32(2), dogResp.Msg.TotalCount)
	s.Len(dogResp.Msg.Pets, 2)

	// Filter by status "AVAILABLE"
	availReq := connect.NewRequest(&petv1.ListPetsRequest{
		Status: petv1.PetStatus_PET_STATUS_AVAILABLE,
	})
	availReq.Header().Set("Authorization", "Bearer test-token")
	availResp, err := s.client.ListPets(s.ctx, availReq)
	s.Require().NoError(err)
	s.Equal(int32(3), availResp.Msg.TotalCount)
	s.Len(availResp.Msg.Pets, 3)

	// Pagination: pageSize=2, page=0
	page0Req := connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 2,
		Page:     0,
	})
	page0Req.Header().Set("Authorization", "Bearer test-token")
	page0Resp, err := s.client.ListPets(s.ctx, page0Req)
	s.Require().NoError(err)
	s.Equal(int32(5), page0Resp.Msg.TotalCount)
	s.Len(page0Resp.Msg.Pets, 2)

	// Pagination: pageSize=2, page=2 (should return 1 pet)
	page2Req := connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 2,
		Page:     2,
	})
	page2Req.Header().Set("Authorization", "Bearer test-token")
	page2Resp, err := s.client.ListPets(s.ctx, page2Req)
	s.Require().NoError(err)
	s.Equal(int32(5), page2Resp.Msg.TotalCount)
	s.Len(page2Resp.Msg.Pets, 1)
}

func TestPetServiceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PetServiceIntegrationTestSuite))
}
