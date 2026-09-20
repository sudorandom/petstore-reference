package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/config"
	"github.com/example/pets/internal/db"
	"github.com/example/pets/internal/pet"
	"github.com/example/pets/internal/telemetry"
	"github.com/sudorandom/protojsonx/protojsonxconnect"
)

func main() {
	cfg := config.Load()
	if cfg.AuthEnabled && !cfg.DevMode && !cfg.TrustProxyHeaders && len(cfg.AuthTokens) == 0 {
		log.Fatal("Authentication is enabled, but neither TRUST_PROXY_HEADERS nor AUTH_TOKENS is configured")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting Pet Microservice on port %s...", cfg.Port)

	// Initialize OpenTelemetry
	otelCfg := telemetry.LoadConfigFromEnv()
	shutdownOTel, err := telemetry.Init(ctx, otelCfg)
	if err != nil {
		log.Printf("Warning: OpenTelemetry init failed: %v", err)
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := shutdownOTel(shutdownCtx); err != nil {
				log.Printf("Error shutting down OpenTelemetry: %v", err)
			}
		}()
	}

	// Initialize Database Pool
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL successfully.")

	if cfg.AutoMigrate {
		log.Println("Applying database migrations (AUTO_MIGRATE=true)...")
		if err := db.Migrate(ctx, pool); err != nil {
			log.Fatalf("Failed to apply database migrations: %v", err)
		}
	} else {
		log.Println("Skipping automatic database migrations (AUTO_MIGRATE=false). Use 'migrate' CLI for migrations.")
	}

	handler, err := newServerHandler(cfg, pool)
	if err != nil {
		log.Fatalf("Failed to initialize server handler: %v", err)
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
			log.Printf("Pet Microservice listening on https://localhost:%s (TLS enabled via mkcert)", cfg.Port)
			if err := srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("Server TLS error: %v", err)
			}
		} else {
			log.Printf("Pet Microservice listening on http://localhost:%s (cleartext)", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("Server error: %v", err)
			}
		}
	}()

	<-stop
	log.Println("Shutting down Pet Microservice...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}

func newServerHandler(cfg *config.Config, pool *pgxpool.Pool) (http.Handler, error) {
	queries := db.New(pool)
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

	photoHandler := otelhttp.NewHandler(pet.NewPhotoHandler(queries), "photos")
	mux.Handle("GET /photos/{id}", auth.Middleware(authCfg)(photoHandler))

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
