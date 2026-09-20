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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Enabled:           true,
		DevMode:           false,
		StaticTokens:      []string{"valid-token-123"},
		TrustProxyHeaders: true,
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
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("Google Cloud IAP headers", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Goog-Authenticated-User-Email", "accounts.google.com:alice@example.com")
		req.Header().Set("X-Goog-Authenticated-User-Id", "accounts.google.com:10987654321")
		resp, err := client.GetPet(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "TestPet", resp.Msg.Pet.Name)
		require.NotNil(t, svc.lastClaims)
		assert.Equal(t, "alice@example.com", svc.lastClaims.Email)
		assert.Equal(t, "iap", svc.lastClaims.Provider)
	})

	t.Run("OAuth2 Proxy headers", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Forwarded-Email", "bob@example.com")
		req.Header().Set("X-Forwarded-User", "bob123")
		req.Header().Set("X-Forwarded-Groups", "engineering, devops")
		resp, err := client.GetPet(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "TestPet", resp.Msg.Pet.Name)
		require.NotNil(t, svc.lastClaims)
		assert.Equal(t, "bob@example.com", svc.lastClaims.Email)
		assert.Equal(t, "oauth2-proxy", svc.lastClaims.Provider)
		assert.Equal(t, []string{"engineering", "devops"}, svc.lastClaims.Roles)
	})

	t.Run("valid service-to-service bearer token", func(t *testing.T) {
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("Authorization", "Bearer valid-token-123")
		_, err := client.GetPet(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, svc.lastClaims)
		assert.Equal(t, "bearer", svc.lastClaims.Provider)
	})

	t.Run("skipped public procedure", func(t *testing.T) {
		req := connect.NewRequest(&petv1.ListPetsRequest{})
		resp, err := client.ListPets(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg)
	})

	t.Run("validator rejects missing IAP JWT assertion", func(t *testing.T) {
		valCfg := auth.Config{
			Enabled:           true,
			TrustProxyHeaders: true,
			Validator: func(ctx context.Context, token string) (*auth.Claims, error) {
				return &auth.Claims{Email: "verified@example.com", Provider: "iap"}, nil
			},
		}
		valPath, valHandler := petv1connect.NewPetServiceHandler(
			svc,
			connect.WithInterceptors(auth.NewInterceptor(valCfg)),
		)
		valMux := http.NewServeMux()
		valMux.Handle(valPath, valHandler)
		valServer := httptest.NewServer(valMux)
		defer valServer.Close()

		valClient := petv1connect.NewPetServiceClient(valServer.Client(), valServer.URL)
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Goog-Authenticated-User-Email", "accounts.google.com:unverified@example.com")
		// Missing X-Goog-IAP-JWT-Assertion
		_, err := valClient.GetPet(ctx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("validator accepts valid IAP JWT assertion", func(t *testing.T) {
		valCfg := auth.Config{
			Enabled:           true,
			TrustProxyHeaders: true,
			Validator: func(ctx context.Context, token string) (*auth.Claims, error) {
				return &auth.Claims{Email: "verified@example.com", Provider: "iap"}, nil
			},
		}
		valPath, valHandler := petv1connect.NewPetServiceHandler(
			svc,
			connect.WithInterceptors(auth.NewInterceptor(valCfg)),
		)
		valMux := http.NewServeMux()
		valMux.Handle(valPath, valHandler)
		valServer := httptest.NewServer(valMux)
		defer valServer.Close()

		valClient := petv1connect.NewPetServiceClient(valServer.Client(), valServer.URL)
		req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
		req.Header().Set("X-Goog-IAP-JWT-Assertion", "valid-jwt-token")
		resp, err := valClient.GetPet(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg)
		assert.Equal(t, "verified@example.com", svc.lastClaims.Email)
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
		require.NoError(t, err)
		require.NotNil(t, svc.lastClaims)
		assert.Equal(t, "dev", svc.lastClaims.Provider)
	})
}

func TestAuthInterceptorRejectsUntrustedProxyHeaders(t *testing.T) {
	svc := &mockPetService{}
	path, handler := petv1connect.NewPetServiceHandler(
		svc,
		connect.WithInterceptors(auth.NewInterceptor(auth.Config{Enabled: true})),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := petv1connect.NewPetServiceClient(server.Client(), server.URL)
	req := connect.NewRequest(&petv1.GetPetRequest{Id: "123e4567-e89b-12d3-a456-426614174000"})
	req.Header().Set("X-Forwarded-Email", "attacker@example.com")
	req.Header().Set("X-Forwarded-User", "attacker")

	_, err := client.GetPet(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
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

		assert.Equal(t, "accounts.google.com:local-dev@example.com", capturedReq.Header.Get("X-Goog-Authenticated-User-Email"))
		assert.Equal(t, "local-dev@example.com", capturedReq.Header.Get("X-Forwarded-Email"))
		assert.Equal(t, "local-dev-user", capturedReq.Header.Get("X-Forwarded-User"))
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

		assert.Equal(t, "accounts.google.com:custom@example.com", capturedReq.Header.Get("X-Goog-Authenticated-User-Email"))
		assert.Empty(t, capturedReq.Header.Get("X-Forwarded-Email"))
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

		assert.Empty(t, capturedReq.Header.Get("X-Goog-Authenticated-User-Email"))
		assert.Equal(t, "Bearer custom-token", capturedReq.Header.Get("Authorization"))
	})
}
