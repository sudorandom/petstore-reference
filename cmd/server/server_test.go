package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/pets/internal/config"
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
