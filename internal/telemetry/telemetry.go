package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
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
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	Insecure       bool
	ExporterType   string // "otlp", "stdout", "none"
}

// LoadConfigFromEnv builds a Config from standard environment variables.
func LoadConfigFromEnv() Config {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "pets-service"
	}

	serviceVersion := os.Getenv("OTEL_SERVICE_VERSION")
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	exporterType := strings.ToLower(os.Getenv("OTEL_TRACES_EXPORTER"))
	if exporterType == "" {
		if otlpEndpoint != "" {
			exporterType = "otlp"
		} else {
			exporterType = "none"
		}
	}

	insecure := true
	if val := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); val != "" {
		insecure = strings.ToLower(val) == "true" || val == "1"
	}

	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   otlpEndpoint,
		Insecure:       insecure,
		ExporterType:   exporterType,
	}
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
