package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	AuthEnabled bool
	DevMode     bool
	DevEmail    string
	AuthTokens  []string
	CertFile    string
	KeyFile     string
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

	devEmail := getEnv("DEV_EMAIL", "developer@local.test")

	authEnabled := true
	if val := os.Getenv("AUTH_ENABLED"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			authEnabled = parsed
		}
	}

	tokensStr := getEnv("AUTH_TOKENS", "dev-secret-token")
	tokens := strings.Split(tokensStr, ",")
	for i := range tokens {
		tokens[i] = strings.TrimSpace(tokens[i])
	}

	certFile := getEnv("TLS_CERT_FILE", ".certs/cert.pem")
	keyFile := getEnv("TLS_KEY_FILE", ".certs/key.pem")

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		AuthEnabled: authEnabled,
		DevMode:     devMode,
		DevEmail:    devEmail,
		AuthTokens:  tokens,
		CertFile:    certFile,
		KeyFile:     keyFile,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
