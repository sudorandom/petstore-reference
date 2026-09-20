package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/ilyakaznacheev/cleanenv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds configuration options for OpenTelemetry.
type Config struct {
	ServiceName      string  `env:"OTEL_SERVICE_NAME" yaml:"service_name" json:"service_name"`
	ServiceVersion   string  `env:"OTEL_SERVICE_VERSION" yaml:"service_version" json:"service_version"`
	OTLPEndpoint     string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT" yaml:"otlp_endpoint" json:"otlp_endpoint"`
	Insecure         bool    `env:"OTEL_EXPORTER_OTLP_INSECURE" yaml:"insecure" json:"insecure"`
	ExporterType     string  `env:"OTEL_TRACES_EXPORTER" yaml:"exporter_type" json:"exporter_type"`
	SamplePercentage float64 `env:"OTEL_SAMPLE_PERCENTAGE" yaml:"sample_percentage" json:"sample_percentage"`
}

// LoadConfigFromEnv builds a Config from environment variables and optional configuration files (YAML, JSON, TOML).
// If configPath is provided or OTEL_CONFIG_FILE / CONFIG_FILE is set, the file is read and then overridden by environment variables.
func LoadConfigFromEnv(configPath ...string) Config {
	cfg := Config{
		ServiceName:      "pets-service",
		ServiceVersion:   "1.0.0",
		Insecure:         true,
		SamplePercentage: 100.0,
	}

	// Sanitize environment variables for cleanenv
	if val, ok := os.LookupEnv("OTEL_SAMPLE_PERCENTAGE"); ok {
		if before, ok0 := strings.CutSuffix(val, "%"); ok0 {
			_ = os.Setenv("OTEL_SAMPLE_PERCENTAGE", before)
		}
	} else if val := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); val != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			if parsed <= 1.0 && parsed > 0 {
				parsed = parsed * 100.0
			}
			_ = os.Setenv("OTEL_SAMPLE_PERCENTAGE", strconv.FormatFloat(parsed, 'f', -1, 64))
			defer func() { _ = os.Unsetenv("OTEL_SAMPLE_PERCENTAGE") }()
		}
	}

	targetPath := ""
	if len(configPath) > 0 && configPath[0] != "" {
		targetPath = configPath[0]
	} else if envPath := os.Getenv("OTEL_CONFIG_FILE"); envPath != "" {
		targetPath = envPath
	} else if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
		targetPath = envPath
	}

	if targetPath != "" {
		targetPath = filepath.Clean(targetPath)
		if _, err := os.Stat(targetPath); err == nil { //nolint:gosec // targetPath is from CLI flag or env var
			if err := cleanenv.ReadConfig(targetPath, &cfg); err != nil {
				slog.Warn("Failed to read telemetry config file, falling back to environment variables",
					"path", targetPath,
					"error", err,
				)
				_ = cleanenv.ReadEnv(&cfg)
			}
		} else {
			_ = cleanenv.ReadEnv(&cfg)
		}
	} else {
		_ = cleanenv.ReadEnv(&cfg)
	}

	if cfg.ExporterType == "" {
		if cfg.OTLPEndpoint != "" {
			cfg.ExporterType = "otlp"
		} else {
			cfg.ExporterType = "none"
		}
	}

	return cfg
}

// Init initializes the OpenTelemetry TracerProvider and global propagators.
// It returns a shutdown function that flushes and cleans up the TracerProvider.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// Set global W3C TraceContext and Baggage propagators.
	// This enables distributed tracing across frontend, connect-rpc, and downstream services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	var opts []sdktrace.TracerProviderOption
	opts = append(opts, sdktrace.WithResource(res))

	// Configure sampler: ParentBased ensures remote parent sampling decisions are respected.
	// For root spans (no parent), the sample percentage is used.
	var rootSampler sdktrace.Sampler
	switch {
	case cfg.SamplePercentage >= 100.0:
		rootSampler = sdktrace.AlwaysSample()
	case cfg.SamplePercentage <= 0.0:
		rootSampler = sdktrace.NeverSample()
	default:
		rootSampler = sdktrace.TraceIDRatioBased(cfg.SamplePercentage / 100.0)
	}
	opts = append(opts, sdktrace.WithSampler(sdktrace.ParentBased(rootSampler)))

	switch cfg.ExporterType {
	case "stdout":
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	case "otlp":
		var grpcOpts []otlptracegrpc.Option
		if cfg.OTLPEndpoint != "" {
			grpcOpts = append(grpcOpts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.Insecure {
			grpcOpts = append(grpcOpts, otlptracegrpc.WithInsecure())
		}
		exporter, err := otlptracegrpc.New(ctx, grpcOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlp grpc trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	case "none":
		// No exporter, but tracer provider remains active in memory for context propagation and span creation
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// NewConnectInterceptor creates a ConnectRPC interceptor instrumented with OpenTelemetry.
// It is configured with WithTrustRemote to adopt frontend-initiated trace contexts (W3C traceparent),
// and WithPropagateResponseHeader to include trace context in RPC responses.
func NewConnectInterceptor() (connect.Interceptor, error) {
	return otelconnect.NewInterceptor(
		otelconnect.WithTrustRemote(),
		otelconnect.WithPropagateResponseHeader(),
	)
}
