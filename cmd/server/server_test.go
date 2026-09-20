package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/cors"
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
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 or 204, got %d", resp.StatusCode)
	}

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://localhost:4321" {
		t.Errorf("expected Access-Control-Allow-Origin: https://localhost:4321, got %q", allowOrigin)
	}

	allowCreds := resp.Header.Get("Access-Control-Allow-Credentials")
	if allowCreds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true, got %q", allowCreds)
	}
}
