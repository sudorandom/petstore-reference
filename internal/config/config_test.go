package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadProductionHasNoDefaultCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_MODE", "")
	t.Setenv("AUTH_TOKENS", "")
	t.Setenv("TRUST_PROXY_HEADERS", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg := Load()

	assert.False(t, cfg.DevMode)
	assert.Empty(t, cfg.AuthTokens)
	assert.False(t, cfg.TrustProxyHeaders)
	assert.Empty(t, cfg.AllowedOrigins)
}

func TestLoadExplicitSecurityConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_MODE", "")
	t.Setenv("AUTH_TOKENS", " first-token, ,second-token ")
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")

	cfg := Load()

	assert.Equal(t, []string{"first-token", "second-token"}, cfg.AuthTokens)
	assert.True(t, cfg.TrustProxyHeaders)
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, cfg.AllowedOrigins)
}

func TestLoadRejectsWildcardCredentialedOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_MODE", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	assert.Empty(t, Load().AllowedOrigins)
}
