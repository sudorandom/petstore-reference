package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/testcontainers/testcontainers-go"
	pgmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/pet"
	"github.com/example/pets/internal/validator"
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

func TestPostgres_PetService_Integration(t *testing.T) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		pgContainer, err := pgmodule.Run(ctx,
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
			t.Skipf("Skipping integration test: Docker/Testcontainers not available: %v", err)
			return
		}
		defer func() {
			_ = testcontainers.TerminateContainer(pgContainer)
		}()

		dbURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatalf("failed to get connection string from testcontainer: %v", err)
		}
	}

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database at %s: %v", dbURL, err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("failed to apply database migrations: %v", err)
	}

	pv, err := protovalidate.New()
	if err != nil {
		t.Fatalf("failed to create protovalidate: %v", err)
	}

	queries := db.New(pool)
	service := pet.NewService(queries)

	valInterceptor := validator.NewInterceptor(pv)
	authInterceptor := auth.NewInterceptor(auth.Config{
		Enabled:      true,
		StaticTokens: []string{"test-token"},
	})

	path, handler := petv1connect.NewPetServiceHandler(
		service,
		connect.WithCodec(&protojsonxconnect.Codec{}),
		connect.WithInterceptors(valInterceptor, authInterceptor),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	// ConnectRPC client
	client := petv1connect.NewPetServiceClient(
		server.Client(),
		server.URL,
		connect.WithCodec(&protojsonxconnect.Codec{}),
	)

	var createdPetID string

	t.Run("CreatePet", func(t *testing.T) {
		req := connect.NewRequest(&petv1.CreatePetRequest{
			Name:      "Milo",
			Species:   "Dog",
			Age:       2,
			Status:    petv1.PetStatus_PET_STATUS_AVAILABLE,
			PhotoUrls: []string{"https://example.com/milo.jpg"},
			Tags:      []string{"friendly", "playful"},
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := client.CreatePet(ctx, req)
		if err != nil {
			t.Fatalf("CreatePet failed: %v", err)
		}

		if resp.Msg.Pet.Name != "Milo" {
			t.Errorf("expected pet name Milo, got %s", resp.Msg.Pet.Name)
		}
		if resp.Msg.Pet.Id == "" {
			t.Fatal("expected non-empty pet ID")
		}
		if resp.Msg.Pet.CreatedBy == "" || resp.Msg.Pet.ModifiedBy == "" {
			t.Errorf("expected created_by and modified_by to be populated, got created_by=%q, modified_by=%q", resp.Msg.Pet.CreatedBy, resp.Msg.Pet.ModifiedBy)
		}
		if resp.Msg.Pet.CreatedAt == nil || resp.Msg.Pet.ModifiedAt == nil {
			t.Errorf("expected created_at and modified_at to be non-nil")
		}
		createdPetID = resp.Msg.Pet.Id
	})

	t.Run("GetPet", func(t *testing.T) {
		if createdPetID == "" {
			t.Skip("skipping GetPet because CreatePet did not succeed")
		}

		req := connect.NewRequest(&petv1.GetPetRequest{Id: createdPetID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := client.GetPet(ctx, req)
		if err != nil {
			t.Fatalf("GetPet failed: %v", err)
		}
		if resp.Msg.Pet.Id != createdPetID {
			t.Errorf("expected pet ID %s, got %s", createdPetID, resp.Msg.Pet.Id)
		}
	})

	t.Run("ListPets", func(t *testing.T) {
		req := connect.NewRequest(&petv1.ListPetsRequest{
			Species:  "Dog",
			PageSize: 10,
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := client.ListPets(ctx, req)
		if err != nil {
			t.Fatalf("ListPets failed: %v", err)
		}
		if len(resp.Msg.Pets) == 0 {
			t.Errorf("expected at least 1 pet in list")
		}
	})

	t.Run("UpdatePet", func(t *testing.T) {
		if createdPetID == "" {
			t.Skip("skipping UpdatePet")
		}

		req := connect.NewRequest(&petv1.UpdatePetRequest{
			Id:        createdPetID,
			Name:      "Milo The Great",
			Species:   "Dog",
			Age:       3,
			Status:    petv1.PetStatus_PET_STATUS_ADOPTED,
			PhotoUrls: []string{"https://example.com/milo2.jpg"},
			Tags:      []string{"adopted", "happy"},
		})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := client.UpdatePet(ctx, req)
		if err != nil {
			t.Fatalf("UpdatePet failed: %v", err)
		}
		if resp.Msg.Pet.Name != "Milo The Great" {
			t.Errorf("expected updated name, got %s", resp.Msg.Pet.Name)
		}
		if resp.Msg.Pet.Status != petv1.PetStatus_PET_STATUS_ADOPTED {
			t.Errorf("expected status ADOPTED, got %v", resp.Msg.Pet.Status)
		}
	})

	t.Run("DeletePet", func(t *testing.T) {
		if createdPetID == "" {
			t.Skip("skipping DeletePet")
		}

		req := connect.NewRequest(&petv1.DeletePetRequest{Id: createdPetID})
		req.Header().Set("Authorization", "Bearer test-token")

		resp, err := client.DeletePet(ctx, req)
		if err != nil {
			t.Fatalf("DeletePet failed: %v", err)
		}
		if !resp.Msg.Success {
			t.Errorf("expected success true")
		}

		// Verify pet is gone
		getReq := connect.NewRequest(&petv1.GetPetRequest{Id: createdPetID})
		getReq.Header().Set("Authorization", "Bearer test-token")
		_, err = client.GetPet(ctx, getReq)
		if err == nil {
			t.Fatalf("expected pet to be not found, but GetPet succeeded")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
		}
	})
}
