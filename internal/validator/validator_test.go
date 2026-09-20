package validator_test

import (
	"context"
	"testing"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/internal/validator"
)

func TestValidatorInterceptor(t *testing.T) {
	pv, err := protovalidate.New()
	if err != nil {
		t.Fatalf("failed to create protovalidate validator: %v", err)
	}

	interceptor := validator.NewInterceptor(pv)
	dummyNext := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&petv1.CreatePetResponse{}), nil
	}

	t.Run("valid CreatePetRequest", func(t *testing.T) {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:               "Buddy",
			Species:            "Dog",
			BirthDate:          "2023-01-15",
			BirthDateEstimated: true,
			Status:             petv1.PetStatus_PET_STATUS_AVAILABLE,
		})
		_, err := interceptor(dummyNext)(context.Background(), req)
		if err != nil {
			t.Fatalf("expected valid request to pass, got: %v", err)
		}
	})

	t.Run("invalid CreatePetRequest - missing name", func(t *testing.T) {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:      "",
			Species:   "Dog",
			BirthDate: "2023-01-15",
			Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
		})
		_, err := interceptor(dummyNext)(context.Background(), req)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
		}
	})

	t.Run("invalid CreatePetRequest - invalid birth_date format", func(t *testing.T) {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:      "Buddy",
			Species:   "Dog",
			BirthDate: "invalid-date",
			Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
		})
		_, err := interceptor(dummyNext)(context.Background(), req)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
		}
	})

	t.Run("invalid GetPetRequest - invalid UUID", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{
			Id: "not-a-valid-uuid",
		})
		_, err := interceptor(dummyNext)(context.Background(), req)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
		}
	})
}
