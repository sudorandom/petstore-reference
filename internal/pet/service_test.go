package pet

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPetsRejectsOverflowingPageOffset(t *testing.T) {
	service := NewService(nil)
	req := connect.NewRequest(&petv1.ListPetsRequest{
		PageSize: 100,
		Page:     21_474_837,
	})

	_, err := service.ListPets(context.Background(), req)

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

	_, err = parseDate("invalid-date")
	require.Error(t, err)
}

func TestIsValidImageMime(t *testing.T) {
	assert.True(t, isValidImageMime("image/jpeg", "image/jpeg"))
	assert.True(t, isValidImageMime("image/png", "image/png"))
	assert.True(t, isValidImageMime("image/gif", "image/gif"))
	assert.True(t, isValidImageMime("image/webp", "image/webp"))
	assert.True(t, isValidImageMime("application/octet-stream", "image/webp"))

	assert.False(t, isValidImageMime("text/html", "image/png"))
	assert.False(t, isValidImageMime("image/png", "image/jpeg"))
}
