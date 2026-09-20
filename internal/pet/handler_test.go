package pet

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/testutil"
)

type PetHandlerTestSuite struct {
	suite.Suite
	ctx     context.Context
	cancel  context.CancelFunc
	testDB  *testutil.TestDB
	handler *Handler
}

func (s *PetHandlerTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	testDB, err := testutil.StartTestDB(s.ctx)
	if err != nil {
		s.T().Skipf("Skipping PetHandler tests: postgres testcontainer unavailable: %v", err)
		return
	}
	s.testDB = testDB
	s.handler = NewHandler(testDB.Pool)
}

func (s *PetHandlerTestSuite) TearDownSuite() {
	if s.testDB != nil {
		s.testDB.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *PetHandlerTestSuite) SetupTest() {
	s.Require().NoError(s.testDB.TruncateTables(s.ctx), "failed to truncate tables before test")
}

func (s *PetHandlerTestSuite) authContext(email string) context.Context {
	claims := &auth.Claims{
		Email:    email,
		Subject:  "test-user",
		Provider: "test",
		Roles:    []string{"user"},
	}
	return auth.WithClaims(s.ctx, claims)
}

func (s *PetHandlerTestSuite) TestCreatePet() {
	ctx := s.authContext("creator@example.com")

	req := connect.NewRequest(&petv1.CreatePetRequest{
		Name:               "  Luna  ",
		Species:            "  Cat  ",
		BirthDate:          "2023-06-15",
		BirthDateEstimated: true,
		Status:             petv1.PetStatus_PET_STATUS_UNSPECIFIED, // Should default to AVAILABLE
		Tags:               []string{"calico", "friendly"},
		PhotoUrls:          []string{"https://example.com/luna1.jpg", "https://example.com/luna2.jpg"},
	})

	resp, err := s.handler.CreatePet(ctx, req)
	s.Require().NoError(err)
	s.Require().NotNil(resp.Msg.Pet)

	pet := resp.Msg.Pet
	s.NotEmpty(pet.Id)
	s.Equal("Luna", pet.Name) // Trimmed
	s.Equal("Cat", pet.Species) // Trimmed
	s.Equal("2023-06-15", pet.BirthDate)
	s.True(pet.BirthDateEstimated)
	s.Equal(petv1.PetStatus_PET_STATUS_AVAILABLE, pet.Status) // Defaulted
	s.Equal([]string{"calico", "friendly"}, pet.Tags)
	s.Equal([]string{"https://example.com/luna1.jpg", "https://example.com/luna2.jpg"}, pet.PhotoUrls)
	s.Equal("creator@example.com", pet.CreatedBy)
	s.Equal("creator@example.com", pet.ModifiedBy)

	// Create pet without birth date (optional)
	reqNoBirth := connect.NewRequest(&petv1.CreatePetRequest{
		Name:    "Mochi",
		Species: "Cat",
	})
	respNoBirth, err := s.handler.CreatePet(ctx, reqNoBirth)
	s.Require().NoError(err)
	s.Require().NotNil(respNoBirth.Msg.Pet)
	s.Empty(respNoBirth.Msg.Pet.BirthDate)
}

func (s *PetHandlerTestSuite) TestCreatePetValidation() {
	ctx := s.authContext("creator@example.com")

	// Blank name
	_, err := s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "   ",
		Species:   "Dog",
		BirthDate: "2023-01-01",
	}))
	s.Require().Error(err)
	s.Equal(connect.CodeInvalidArgument, connect.CodeOf(err))

	// Invalid birth date format
	_, err = s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Fido",
		Species:   "Dog",
		BirthDate: "not-a-date",
	}))
	s.Require().Error(err)
	s.Equal(connect.CodeInvalidArgument, connect.CodeOf(err))
}

func (s *PetHandlerTestSuite) TestGetPet() {
	ctx := s.authContext("user@example.com")

	// 1. Not found
	_, err := s.handler.GetPet(ctx, connect.NewRequest(&petv1.GetPetRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	}))
	s.Require().Error(err)
	s.Equal(connect.CodeNotFound, connect.CodeOf(err))

	// 2. Existing pet
	createResp, err := s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Max",
		Species:   "Dog",
		BirthDate: "2021-04-10",
		Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
	}))
	s.Require().NoError(err)

	getResp, err := s.handler.GetPet(ctx, connect.NewRequest(&petv1.GetPetRequest{
		Id: createResp.Msg.Pet.Id,
	}))
	s.Require().NoError(err)
	s.Equal(createResp.Msg.Pet.Id, getResp.Msg.Pet.Id)
	s.Equal("Max", getResp.Msg.Pet.Name)
}

func (s *PetHandlerTestSuite) TestListPets() {
	ctx := s.authContext("user@example.com")

	// Seed 3 pets: 2 Dogs (1 AVAILABLE, 1 ADOPTED), 1 Cat (AVAILABLE)
	dog1Resp, err := s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Dog1",
		Species:   "Dog",
		BirthDate: "2020-01-01",
		Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
		PhotoUrls: []string{"https://example.com/dog1.jpg"},
	}))
	s.Require().NoError(err)
	dog1ID := dog1Resp.Msg.Pet.Id

	_, err = s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name: "Dog2", Species: "Dog", BirthDate: "2021-01-01", Status: petv1.PetStatus_PET_STATUS_ADOPTED,
	}))
	s.Require().NoError(err)

	_, err = s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name: "Cat1", Species: "Cat", BirthDate: "2022-01-01", Status: petv1.PetStatus_PET_STATUS_AVAILABLE,
	}))
	s.Require().NoError(err)

	// Filter by species "Dog"
	dogResp, err := s.handler.ListPets(ctx, connect.NewRequest(&petv1.ListPetsRequest{
		Species: "Dog",
	}))
	s.Require().NoError(err)
	s.Equal(int32(2), dogResp.Msg.TotalCount)
	s.Len(dogResp.Msg.Pets, 2)
	// One of the dogs should have photo_urls
	var foundPhoto bool
	for _, p := range dogResp.Msg.Pets {
		if len(p.PhotoUrls) > 0 {
			foundPhoto = true
			s.Equal(dog1ID, p.Id)
			s.Equal([]string{"https://example.com/dog1.jpg"}, p.PhotoUrls)
		}
	}
	s.True(foundPhoto)

	// Filter by status ADOPTED
	adoptedResp, err := s.handler.ListPets(ctx, connect.NewRequest(&petv1.ListPetsRequest{
		Status: petv1.PetStatus_PET_STATUS_ADOPTED,
	}))
	s.Require().NoError(err)
	s.Equal(int32(1), adoptedResp.Msg.TotalCount)
	s.Len(adoptedResp.Msg.Pets, 1)
	s.Equal("Dog2", adoptedResp.Msg.Pets[0].Name)

	// Pagination: PageSize=1, Page=0 then Page=1
	page0, err := s.handler.ListPets(ctx, connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 1,
		Page:     0,
	}))
	s.Require().NoError(err)
	s.Len(page0.Msg.Pets, 1)

	page1, err := s.handler.ListPets(ctx, connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 1,
		Page:     1,
	}))
	s.Require().NoError(err)
	s.Len(page1.Msg.Pets, 1)
	s.NotEqual(page0.Msg.Pets[0].Id, page1.Msg.Pets[0].Id)
}

func (s *PetHandlerTestSuite) TestUpdatePet() {
	ctx := s.authContext("updater@example.com")

	createResp, err := s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Rocky",
		Species:   "Dog",
		BirthDate: "2022-05-01",
		Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
	}))
	s.Require().NoError(err)

	updateResp, err := s.handler.UpdatePet(ctx, connect.NewRequest(&petv1.UpdatePetRequest{
		Id:                 createResp.Msg.Pet.Id,
		Name:               "Rocky Balboa",
		Species:            "Dog",
		BirthDate:          "2022-05-01",
		BirthDateEstimated: true,
		Status:             petv1.PetStatus_PET_STATUS_ADOPTED,
		Tags:               []string{"champion"},
		PhotoUrls:          []string{"https://example.com/rocky.jpg"},
	}))
	s.Require().NoError(err)
	s.Equal("Rocky Balboa", updateResp.Msg.Pet.Name)
	s.Equal(petv1.PetStatus_PET_STATUS_ADOPTED, updateResp.Msg.Pet.Status)
	s.Equal([]string{"https://example.com/rocky.jpg"}, updateResp.Msg.Pet.PhotoUrls)
	s.Equal("updater@example.com", updateResp.Msg.Pet.ModifiedBy)

	// Update non-existent pet -> CodeNotFound
	_, err = s.handler.UpdatePet(ctx, connect.NewRequest(&petv1.UpdatePetRequest{
		Id:        "00000000-0000-0000-0000-000000000000",
		Name:      "Ghost",
		Species:   "Wolf",
		BirthDate: "2020-01-01",
	}))
	s.Require().Error(err)
	s.Equal(connect.CodeNotFound, connect.CodeOf(err))
}

func (s *PetHandlerTestSuite) TestDeletePet() {
	ctx := s.authContext("user@example.com")

	createResp, err := s.handler.CreatePet(ctx, connect.NewRequest(&petv1.CreatePetRequest{
		Name:      "Charlie",
		Species:   "Parrot",
		BirthDate: "2021-08-20",
		Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
	}))
	s.Require().NoError(err)
	petID := createResp.Msg.Pet.Id

	// Delete existing pet
	delResp, err := s.handler.DeletePet(ctx, connect.NewRequest(&petv1.DeletePetRequest{Id: petID}))
	s.Require().NoError(err)
	s.True(delResp.Msg.Success)

	// Delete again -> CodeNotFound
	_, err = s.handler.DeletePet(ctx, connect.NewRequest(&petv1.DeletePetRequest{Id: petID}))
	s.Require().Error(err)
	s.Equal(connect.CodeNotFound, connect.CodeOf(err))
}

func TestPetHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(PetHandlerTestSuite))
}

// Unit tests without database requirements:

func TestListPetsRejectsOverflowingPageOffset(t *testing.T) {
	handler := NewHandler(nil)
	req := connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 100,
		Page:     21_474_837,
	})

	_, err := handler.ListPets(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestToProtoPetStatusNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected petv1.PetStatus
	}{
		{"AVAILABLE", petv1.PetStatus_PET_STATUS_AVAILABLE},
		{"PET_STATUS_AVAILABLE", petv1.PetStatus_PET_STATUS_AVAILABLE},
		{"PENDING", petv1.PetStatus_PET_STATUS_PENDING},
		{"PET_STATUS_PENDING", petv1.PetStatus_PET_STATUS_PENDING},
		{"ADOPTED", petv1.PetStatus_PET_STATUS_ADOPTED},
		{"PET_STATUS_ADOPTED", petv1.PetStatus_PET_STATUS_ADOPTED},
		{"UNKNOWN_STATUS", petv1.PetStatus_PET_STATUS_UNSPECIFIED},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pet := toProtoPet(db.Pet{Status: tc.input})
			assert.Equal(t, tc.expected, pet.Status)
		})
	}
}

func TestParseDate(t *testing.T) {
	d, err := parseDate("2023-05-10")
	require.NoError(t, err)
	assert.True(t, d.Valid)
	assert.Equal(t, "2023-05-10", d.Time.Format("2006-01-02"))

	emptyD, err := parseDate("")
	require.NoError(t, err)
	assert.False(t, emptyD.Valid)

	blankD, err := parseDate("   ")
	require.NoError(t, err)
	assert.False(t, blankD.Valid)

	_, err = parseDate("invalid-date")
	require.Error(t, err)
}
