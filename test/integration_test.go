package test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/pet"
	"github.com/example/pets/internal/telemetry"
	"github.com/sudorandom/protojsonx/protojsonxconnect"
)

func init() {
	// Automatically detect and configure Colima socket on macOS if DOCKER_HOST is not set
	if os.Getenv("DOCKER_HOST") == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			colimaSock := filepath.Join(home, ".colima", "default", "docker.sock")
			if _, err := os.Stat(colimaSock); err == nil {
				_ = os.Setenv("DOCKER_HOST", "unix://"+colimaSock)
				if os.Getenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE") == "" {
					_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
				}
			}
		}
	}
}

type PetServiceIntegrationTestSuite struct {
	suite.Suite

	ctx              context.Context
	cancel           context.CancelFunc
	pgContainer      *pgmodule.PostgresContainer
	pool             *pgxpool.Pool
	server           *httptest.Server
	client           petv1connect.PetServiceClient
	createdPetID     string
	uploadedPhotoID  string
	uploadedPhotoURL string
}

func (s *PetServiceIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		pgContainer, err := pgmodule.Run(s.ctx,
			"postgres:17-alpine",
			pgmodule.WithDatabase("pets_db"),
			pgmodule.WithUsername("postgres"),
			pgmodule.WithPassword("password"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
			),
		)
		if err != nil {
			s.T().Skipf("Skipping integration test: Docker/Testcontainers not available: %v", err)
			return
		}
		s.pgContainer = pgContainer

		var errConn error
		dbURL, errConn = pgContainer.ConnectionString(s.ctx, "sslmode=disable")
		s.Require().NoError(errConn, "failed to get connection string from testcontainer")
	}

	pool, err := db.NewPool(s.ctx, dbURL)
	s.Require().NoError(err, "failed to connect to database at %s", dbURL)
	s.pool = pool

	err = db.Migrate(s.ctx, pool)
	s.Require().NoError(err, "failed to apply database migrations")

	queries := db.New(pool)
	service := pet.NewService(pool)

	otelInterceptor, err := telemetry.NewConnectInterceptor()
	s.Require().NoError(err, "failed to create otel interceptor")

	valInterceptor := validate.NewInterceptor()
	authInterceptor := auth.NewInterceptor(auth.Config{
		Enabled:      true,
		StaticTokens: []string{"test-token"},
	})

	path, handler := petv1connect.NewPetServiceHandler(
		service,
		connect.WithCodec(&protojsonxconnect.Codec{}),
		connect.WithInterceptors(otelInterceptor, valInterceptor, authInterceptor),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("GET /photos/{id}", pet.NewPhotoHandler(queries))
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
	if s.pool != nil {
		s.pool.Close()
	}
	if s.pgContainer != nil {
		_ = testcontainers.TerminateContainer(s.pgContainer)
	}
	if s.cancel != nil {
		s.cancel()
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
			PhotoUrls:          []string{"https://example.com/milo.jpg"},
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
			PhotoUrls:          []string{"https://example.com/milo2.jpg"},
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
		s.Contains(resp.Msg.Pet.PhotoUrls, s.uploadedPhotoURL)
	})

	s.Run("GetPetPhoto", func() {
		s.Require().NotEmpty(s.uploadedPhotoID, "skipping GetPetPhoto")

		req := connect.NewRequest(&petv1.GetPetPhotoRequest{PhotoId: s.uploadedPhotoID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := s.client.GetPetPhoto(s.ctx, req)
		s.Require().NoError(err, "GetPetPhoto failed")
		s.Require().NotNil(resp.Msg)
		s.Equal(s.uploadedPhotoID, resp.Msg.PhotoId)
		s.Equal(s.createdPetID, resp.Msg.PetId)
		s.Equal("image/jpeg", resp.Msg.MimeType)
		s.Len(resp.Msg.Data, 10)
	})

	s.Run("ServePetPhotoHTTP", func() {
		s.Require().NotEmpty(s.uploadedPhotoURL, "skipping ServePetPhotoHTTP")

		photoURL := s.server.URL + s.uploadedPhotoURL

		// GET request
		httpResp, err := s.server.Client().Get(photoURL)
		s.Require().NoError(err, "HTTP GET photo failed")
		defer httpResp.Body.Close()

		s.Equal(http.StatusOK, httpResp.StatusCode)
		s.Equal("image/jpeg", httpResp.Header.Get("Content-Type"))

		body, err := io.ReadAll(httpResp.Body)
		s.Require().NoError(err, "failed to read response body")
		s.Len(body, 10)

		// HEAD request - handled automatically by http.ServeMux when mounted with GET
		headResp, err := s.server.Client().Head(photoURL)
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
		getReq := connect.NewRequest(&petv1.GetPetPhotoRequest{PhotoId: s.uploadedPhotoID})
		getReq.Header().Set("Authorization", "Bearer test-token")
		_, err = s.client.GetPetPhoto(s.ctx, getReq)
		s.Require().Error(err, "expected photo to be not found")
		s.Equal(connect.CodeNotFound, connect.CodeOf(err))
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

func TestPetServiceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PetServiceIntegrationTestSuite))
}
