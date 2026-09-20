package config

import (
	"os"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	DatabaseURL       string
	AuthEnabled       bool
	DevMode           bool
	DevEmail          string
	AuthTokens        []string
	TrustProxyHeaders bool
	AllowedOrigins    []string
	CertFile          string
	KeyFile           string
	AutoMigrate       bool
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/pets_db?sslmode=disable")

	env := getEnv("APP_ENV", "development")
	devMode := env != "production"
	if val := os.Getenv("DEV_MODE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			devMode = parsed
		}
	}

	autoMigrate := devMode
	if val := os.Getenv("AUTO_MIGRATE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			autoMigrate = parsed
		}
	}

	devEmail := getEnv("DEV_EMAIL", "developer@local.test")

	authEnabled := true
	if val := os.Getenv("AUTH_ENABLED"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			authEnabled = parsed
		}
	}

	tokensStr := os.Getenv("AUTH_TOKENS")
	if tokensStr == "" && devMode {
		tokensStr = "dev-secret-token"
	}
	tokens := splitNonEmpty(tokensStr)

	trustProxyHeaders := false
	if val := os.Getenv("TRUST_PROXY_HEADERS"); val != "" {
		trustProxyHeaders, _ = strconv.ParseBool(val)
	}

	allowedOrigins := splitNonEmpty(os.Getenv("CORS_ALLOWED_ORIGINS"))
	allowedOrigins = slices.DeleteFunc(allowedOrigins, func(origin string) bool {
		return origin == "*"
	})
	if len(allowedOrigins) == 0 && devMode {
		allowedOrigins = []string{"https://localhost:4321", "http://localhost:4321"}
	}

	certFile := getEnv("TLS_CERT_FILE", ".certs/cert.pem")
	keyFile := getEnv("TLS_KEY_FILE", ".certs/key.pem")

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		AuthEnabled:       authEnabled,
		DevMode:           devMode,
		DevEmail:          devEmail,
		AuthTokens:        tokens,
		TrustProxyHeaders: trustProxyHeaders,
		AllowedOrigins:    allowedOrigins,
		CertFile:          certFile,
		KeyFile:           keyFile,
		AutoMigrate:       autoMigrate,
	}
}

func splitNonEmpty(value string) []string {
	var values []string
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
