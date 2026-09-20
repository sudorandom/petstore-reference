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

func TestLoadDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEV_MODE", "")
	t.Setenv("AUTH_TOKENS", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("AUTO_MIGRATE", "")
	t.Setenv("AUTH_ENABLED", "")

	cfg := Load()

	assert.True(t, cfg.DevMode)
	assert.True(t, cfg.AutoMigrate)
	assert.True(t, cfg.AuthEnabled)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "developer@local.test", cfg.DevEmail)
	assert.Equal(t, []string{"dev-secret-token"}, cfg.AuthTokens)
	assert.Equal(t, []string{"https://localhost:4321", "http://localhost:4321"}, cfg.AllowedOrigins)
}

func TestLoadCustomOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom:5432/db")
	t.Setenv("DEV_MODE", "false")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("DEV_EMAIL", "custom@example.com")
	t.Setenv("TLS_CERT_FILE", "custom-cert.pem")
	t.Setenv("TLS_KEY_FILE", "custom-key.pem")

	cfg := Load()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "postgres://custom:5432/db", cfg.DatabaseURL)
	assert.False(t, cfg.DevMode)
	assert.False(t, cfg.AutoMigrate)
	assert.False(t, cfg.AuthEnabled)
	assert.Equal(t, "custom@example.com", cfg.DevEmail)
	assert.Equal(t, "custom-cert.pem", cfg.CertFile)
	assert.Equal(t, "custom-key.pem", cfg.KeyFile)
}
