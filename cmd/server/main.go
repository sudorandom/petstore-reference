package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"

	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/config"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/pet"
	"github.com/example/pets/internal/telemetry"
	"github.com/sudorandom/protojsonx/protojsonxconnect"
)

func setupLogger(cfg *config.Config) {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.LogFormat) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	cfg := config.Load()
	setupLogger(cfg)

	if cfg.AuthEnabled && !cfg.DevMode && !cfg.TrustProxyHeaders && len(cfg.AuthTokens) == 0 {
		slog.Error("Authentication is enabled, but neither TRUST_PROXY_HEADERS nor AUTH_TOKENS is configured")
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting Pet Microservice", "port", cfg.Port, "dev_mode", cfg.DevMode)

	// Initialize OpenTelemetry
	otelCfg := telemetry.LoadConfigFromEnv()
	shutdownOTel, err := telemetry.Init(ctx, otelCfg)
	if err != nil {
		slog.Warn("OpenTelemetry init failed", "error", err)
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := shutdownOTel(shutdownCtx); err != nil {
				slog.Error("Error shutting down OpenTelemetry", "error", err)
			}
		}()
	}

	// Initialize Database Pool
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("Connected to PostgreSQL successfully")

	if cfg.AutoMigrate {
		slog.Info("Applying database migrations", "auto_migrate", true)
		if err := db.Migrate(ctx, pool); err != nil {
			slog.Error("Failed to apply database migrations", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("Skipping automatic database migrations", "auto_migrate", false)
	}

	handler, err := newServerHandler(cfg, pool)
	if err != nil {
		slog.Error("Failed to initialize server handler", "error", err)
		os.Exit(1)
	}

	// Support HTTP/1.1 and HTTP/2 (TLS & h2c) natively via Go http.Protocols
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           handler,
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Server shutdown signaling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		hasTLS := false
		if _, err := os.Stat(cfg.CertFile); err == nil {
			if _, err := os.Stat(cfg.KeyFile); err == nil {
				hasTLS = true
			}
		}

		if hasTLS {
			slog.Info("Pet Microservice listening", "url", "https://localhost:"+cfg.Port, "tls", true)
			if err := srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Server TLS error", "error", err)
				os.Exit(1)
			}
		} else {
			slog.Info("Pet Microservice listening", "url", "http://localhost:"+cfg.Port, "tls", false)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Server error", "error", err)
				os.Exit(1)
			}
		}
	}()

	<-stop
	slog.Info("Shutting down Pet Microservice...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited cleanly")
}

func newServerHandler(cfg *config.Config, pool *pgxpool.Pool) (http.Handler, error) {
	petHandler := pet.NewHandler(pool)

	otelInterceptor, err := telemetry.NewConnectInterceptor()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenTelemetry Connect interceptor: %w", err)
	}
	valInterceptor := validate.NewInterceptor()
	authCfg := auth.Config{
		Enabled:           cfg.AuthEnabled,
		DevMode:           cfg.DevMode,
		StaticTokens:      cfg.AuthTokens,
		TrustProxyHeaders: cfg.TrustProxyHeaders || cfg.DevMode,
		SkipProcedures:    map[string]bool{},
	}
	authInterceptor := auth.NewInterceptor(authCfg)

	mux := http.NewServeMux()

	petPath, connectHandler := petv1connect.NewPetServiceHandler(
		petHandler,
		connect.WithCodec(&protojsonxconnect.Codec{}),
		connect.WithInterceptors(otelInterceptor, valInterceptor, authInterceptor),
	)
	mux.Handle(petPath, connectHandler)

	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		candidates := []string{
			"gen/openapi/pet/v1/pet.openapi.yaml",
			"../../gen/openapi/pet/v1/pet.openapi.yaml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				http.ServeFile(w, r, c)
				return
			}
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head>
    <title>Petstore API Reference - OpenAPI</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.yaml"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`))
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pool != nil && pool.Ping(r.Context()) == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","database":"connected"}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable","database":"disconnected"}`))
	})


	var rootHandler http.Handler = mux

	if cfg.DevMode {
		rootHandler = auth.DevIdentityMiddleware(cfg.DevEmail, "dev-user-001")(rootHandler)
	}

	return cors.New(corsOptions(cfg)).Handler(rootHandler), nil
}

func corsOptions(cfg *config.Config) cors.Options {
	return cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Grpc-Timeout",
			"Traceparent",
			"Tracestate",
		},
		ExposedHeaders: []string{
			"Content-Encoding",
			"Connect-Content-Encoding",
			"Grpc-Status",
			"Grpc-Message",
			"traceparent",
			"tracestate",
		},
		MaxAge: 300,
	}
}
