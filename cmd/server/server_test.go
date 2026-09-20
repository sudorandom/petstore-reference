package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/pets/internal/config"
	"github.com/example/pets/internal/testutil"
	"github.com/rs/cors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS_PreflightWithCredentials(t *testing.T) {
	c := cors.New(corsOptions(&config.Config{AllowedOrigins: []string{"https://localhost:4321"}}))

	req := httptest.NewRequest(http.MethodOptions, "/pet.v1.PetService/ListPets", nil)
	req.Header.Set("Origin", "https://localhost:4321")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "connect-protocol-version,content-type")

	w := httptest.NewRecorder()
	c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	resp := w.Result()
	require.True(t, resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK)

	assert.Equal(t, "https://localhost:4321", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", resp.Header.Get("Access-Control-Allow-Credentials"))
}

func TestCORS_RejectsUnconfiguredOrigin(t *testing.T) {
	c := cors.New(corsOptions(&config.Config{AllowedOrigins: []string{"https://app.example.com"}}))
	req := httptest.NewRequest(http.MethodOptions, "/pet.v1.PetService/ListPets", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEqual(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestServerHandlerEndpoints(t *testing.T) {
	cfg := &config.Config{
		AuthEnabled:       true,
		DevMode:           true,
		DevEmail:          "dev@example.com",
		AuthTokens:        []string{"token-123"},
		AllowedOrigins:    []string{"https://localhost:4321"},
		TrustProxyHeaders: true,
	}

	// 1. Test with nil pool (healthz should report unavailable)
	handlerNilPool, err := newServerHandler(cfg, nil)
	require.NoError(t, err)

	recHealthzNil := httptest.NewRecorder()
	handlerNilPool.ServeHTTP(recHealthzNil, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recHealthzNil.Code)
	assert.Contains(t, recHealthzNil.Body.String(), `"database":"disconnected"`)

	// 2. Test with real pool via testutil.StartTestDB
	testDB, err := testutil.StartTestDB(context.Background())
	if err == nil {
		defer testDB.Close()
		handler, err := newServerHandler(cfg, testDB.Pool)
		require.NoError(t, err)

		recHealthz := httptest.NewRecorder()
		handler.ServeHTTP(recHealthz, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		assert.Equal(t, http.StatusOK, recHealthz.Code)
		assert.Contains(t, recHealthz.Body.String(), `"database":"connected"`)
	}

	// 3. Test docs endpoint
	recDocs := httptest.NewRecorder()
	handlerNilPool.ServeHTTP(recDocs, httptest.NewRequest(http.MethodGet, "/docs", nil))
	assert.Equal(t, http.StatusOK, recDocs.Code)
	assert.Contains(t, recDocs.Body.String(), "Petstore API Reference - OpenAPI")

	// 4. Test openapi.yaml endpoint
	recOpenAPI := httptest.NewRecorder()
	handlerNilPool.ServeHTTP(recOpenAPI, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	assert.Equal(t, http.StatusOK, recOpenAPI.Code)
}
