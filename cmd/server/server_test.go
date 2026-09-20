package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/cors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS_PreflightWithCredentials(t *testing.T) {
	c := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{"*"},
	})

	req := httptest.NewRequest(http.MethodOptions, "/pet.v1.PetService/ListPets", nil)
	req.Header.Set("Origin", "https://localhost:4321")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Connect-Protocol-Version")

	w := httptest.NewRecorder()
	c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, req)

	resp := w.Result()
	require.True(t, resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK)

	assert.Equal(t, "https://localhost:4321", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", resp.Header.Get("Access-Control-Allow-Credentials"))
}
