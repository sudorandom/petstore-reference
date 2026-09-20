package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
)

type mockPetService struct {
	petv1connect.UnimplementedPetServiceHandler
	lastClaims *auth.Claims
}

func (m *mockPetService) GetPet(ctx context.Context, req *connect.Request[petv1.GetPetRequest]) (*connect.Response[petv1.GetPetResponse], error) {
	if claims, ok := auth.FromContext(ctx); ok {
		m.lastClaims = claims
	}
	return connect.NewResponse(&petv1.GetPetResponse{
		Pet: &petv1.Pet{
			Id:   req.Msg.Id,
			Name: "TestPet",
		},
	}), nil
}

func (m *mockPetService) ListPets(ctx context.Context, req *connect.Request[petv1.ListPetsRequest]) (*connect.Response[petv1.ListPetsResponse], error) {
	return connect.NewResponse(&petv1.ListPetsResponse{}), nil
}

func TestAuthInterceptor(t *testing.T) {
	cfg := auth.Config{
		Enabled:      true,
		DevMode:      false,
		StaticTokens: []string{"valid-token-123"},
		SkipProcedures: map[string]bool{
			petv1connect.PetServiceListPetsProcedure: true,
		},
	}

	svc := &mockPetService{}
	path, handler := petv1connect.NewPetServiceHandler(
		svc,
		connect.WithInterceptors(auth.NewInterceptor(cfg)),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := petv1connect.NewPetServiceClient(server.Client(), server.URL)
	ctx := context.Background()

	t.Run("missing proxy headers or token", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		_, err := client.GetPet(ctx, req)
		if err == nil {
			t.Fatal("expected unauthenticated error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got %v", connect.CodeOf(err))
		}
	})

	t.Run("Google Cloud IAP headers", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Goog-Authenticated-User-Email", "accounts.google.com:alice@example.com")
		req.Header().Set("X-Goog-Authenticated-User-Id", "accounts.google.com:10987654321")
		resp, err := client.GetPet(ctx, req)
		if err != nil {
			t.Fatalf("expected success with IAP headers, got %v", err)
		}
		if resp.Msg.Pet.Name != "TestPet" {
			t.Errorf("expected pet name TestPet, got %s", resp.Msg.Pet.Name)
		}
		if svc.lastClaims == nil || svc.lastClaims.Email != "alice@example.com" {
			t.Errorf("expected email alice@example.com, got %+v", svc.lastClaims)
		}
		if svc.lastClaims.Provider != "iap" {
			t.Errorf("expected provider iap, got %s", svc.lastClaims.Provider)
		}
	})

	t.Run("OAuth2 Proxy headers", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Forwarded-Email", "bob@example.com")
		req.Header().Set("X-Forwarded-User", "bob123")
		req.Header().Set("X-Forwarded-Groups", "engineering, devops")
		resp, err := client.GetPet(ctx, req)
		if err != nil {
			t.Fatalf("expected success with OAuth2 Proxy headers, got %v", err)
		}
		if resp.Msg.Pet.Name != "TestPet" {
			t.Errorf("expected pet name TestPet, got %s", resp.Msg.Pet.Name)
		}
		if svc.lastClaims == nil || svc.lastClaims.Email != "bob@example.com" {
			t.Errorf("expected email bob@example.com, got %+v", svc.lastClaims)
		}
		if svc.lastClaims.Provider != "oauth2-proxy" {
			t.Errorf("expected provider oauth2-proxy, got %s", svc.lastClaims.Provider)
		}
		if len(svc.lastClaims.Roles) != 2 || svc.lastClaims.Roles[0] != "engineering" {
			t.Errorf("expected engineering role, got %+v", svc.lastClaims.Roles)
		}
	})

	t.Run("valid service-to-service bearer token", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("Authorization", "Bearer valid-token-123")
		_, err := client.GetPet(ctx, req)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if svc.lastClaims.Provider != "bearer" {
			t.Errorf("expected provider bearer, got %s", svc.lastClaims.Provider)
		}
	})

	t.Run("skipped public procedure", func(t *testing.T) {
		req := connect.NewRequest(&petv1.ListPetsRequest{})
		resp, err := client.ListPets(ctx, req)
		if err != nil {
			t.Fatalf("expected public endpoint to succeed, got %v", err)
		}
		if resp.Msg == nil {
			t.Error("expected non-nil response")
		}
	})

	t.Run("dev mode automatic identity fallback", func(t *testing.T) {
		devCfg := auth.Config{
			Enabled: true,
			DevMode: true,
		}
		devPath, devHandler := petv1connect.NewPetServiceHandler(
			svc,
			connect.WithInterceptors(auth.NewInterceptor(devCfg)),
		)
		devMux := http.NewServeMux()
		devMux.Handle(devPath, devHandler)
		devServer := httptest.NewServer(devMux)
		defer devServer.Close()

		devClient := petv1connect.NewPetServiceClient(devServer.Client(), devServer.URL)
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		// No headers sent whatsoever
		_, err := devClient.GetPet(ctx, req)
		if err != nil {
			t.Fatalf("expected dev mode to succeed without credentials, got %v", err)
		}
		if svc.lastClaims == nil || svc.lastClaims.Provider != "dev" {
			t.Errorf("expected dev provider claims, got %+v", svc.lastClaims)
		}
	})
}

func TestDevIdentityMiddleware(t *testing.T) {
	middleware := auth.DevIdentityMiddleware("local-dev@example.com", "local-dev-user")

	t.Run("injects headers when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		var capturedReq *http.Request
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)

		if got := capturedReq.Header.Get("X-Goog-Authenticated-User-Email"); got != "accounts.google.com:local-dev@example.com" {
			t.Errorf("expected IAP email header, got %s", got)
		}
		if got := capturedReq.Header.Get("X-Forwarded-Email"); got != "local-dev@example.com" {
			t.Errorf("expected forwarded email header, got %s", got)
		}
		if got := capturedReq.Header.Get("X-Forwarded-User"); got != "local-dev-user" {
			t.Errorf("expected forwarded user header, got %s", got)
		}
	})

	t.Run("preserves existing IAP headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:custom@example.com")
		rec := httptest.NewRecorder()

		var capturedReq *http.Request
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)

		if got := capturedReq.Header.Get("X-Goog-Authenticated-User-Email"); got != "accounts.google.com:custom@example.com" {
			t.Errorf("expected custom email preserved, got %s", got)
		}
		if got := capturedReq.Header.Get("X-Forwarded-Email"); got != "" {
			t.Errorf("expected forwarded email to remain unset, got %s", got)
		}
	})

	t.Run("preserves existing Authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer custom-token")
		rec := httptest.NewRecorder()

		var capturedReq *http.Request
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rec, req)

		if got := capturedReq.Header.Get("X-Goog-Authenticated-User-Email"); got != "" {
			t.Errorf("expected IAP email to remain unset, got %s", got)
		}
		if got := capturedReq.Header.Get("Authorization"); got != "Bearer custom-token" {
			t.Errorf("expected Authorization header preserved, got %s", got)
		}
	})
}
