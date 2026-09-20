package telemetry

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
)

func TestLoadConfigFromEnv(t *testing.T) {
	os.Unsetenv("OTEL_SERVICE_NAME")
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Unsetenv("OTEL_TRACES_EXPORTER")
	os.Unsetenv("OTEL_SAMPLE_PERCENTAGE")
	os.Unsetenv("OTEL_TRACES_SAMPLER_ARG")

	cfg := LoadConfigFromEnv()
	assert.Equal(t, "pets-service", cfg.ServiceName)
	assert.Equal(t, "none", cfg.ExporterType)
	assert.Equal(t, 100.0, cfg.SamplePercentage)

	// Test custom env
	_ = os.Setenv("OTEL_SERVICE_NAME", "custom-petstore")
	_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	_ = os.Setenv("OTEL_SAMPLE_PERCENTAGE", "25%")
	defer func() {
		os.Unsetenv("OTEL_SERVICE_NAME")
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		os.Unsetenv("OTEL_SAMPLE_PERCENTAGE")
		os.Unsetenv("OTEL_TRACES_SAMPLER_ARG")
	}()

	cfg = LoadConfigFromEnv()
	assert.Equal(t, "custom-petstore", cfg.ServiceName)
	assert.Equal(t, "otlp", cfg.ExporterType)
	assert.Equal(t, 25.0, cfg.SamplePercentage)

	// Test OTEL_SAMPLE_PERCENTAGE=0
	_ = os.Setenv("OTEL_SAMPLE_PERCENTAGE", "0")
	cfg = LoadConfigFromEnv()
	assert.Equal(t, 0.0, cfg.SamplePercentage)

	// Test OTEL_TRACES_SAMPLER_ARG=0.5 ratio
	os.Unsetenv("OTEL_SAMPLE_PERCENTAGE")
	_ = os.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.5")
	cfg = LoadConfigFromEnv()
	assert.Equal(t, 50.0, cfg.SamplePercentage)
}

func TestLoadConfig_FromConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. YAML Config File
	yamlFile := filepath.Join(tmpDir, "telemetry.yaml")
	yamlContent := `
service_name: "yaml-pets-service"
service_version: "2.1.0"
exporter_type: "stdout"
sample_percentage: 45.5
insecure: false
`
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0o600))

	cfg := LoadConfigFromEnv(yamlFile)
	assert.Equal(t, "yaml-pets-service", cfg.ServiceName)
	assert.Equal(t, "2.1.0", cfg.ServiceVersion)
	assert.Equal(t, "stdout", cfg.ExporterType)
	assert.False(t, cfg.Insecure)
	assert.Equal(t, 45.5, cfg.SamplePercentage)

	// 2. Override config file with environment variable
	t.Setenv("OTEL_SERVICE_NAME", "env-override-service")
	t.Setenv("OTEL_SAMPLE_PERCENTAGE", "75")
	cfgOverridden := LoadConfigFromEnv(yamlFile)
	assert.Equal(t, "env-override-service", cfgOverridden.ServiceName)
	assert.Equal(t, 75.0, cfgOverridden.SamplePercentage)
	// Other fields from YAML remain intact
	assert.Equal(t, "2.1.0", cfgOverridden.ServiceVersion)
	assert.Equal(t, "stdout", cfgOverridden.ExporterType)

	// 3. Config file via OTEL_CONFIG_FILE env var
	t.Setenv("OTEL_CONFIG_FILE", yamlFile)
	os.Unsetenv("OTEL_SERVICE_NAME")
	os.Unsetenv("OTEL_SAMPLE_PERCENTAGE")
	cfgFromEnvFile := LoadConfigFromEnv()
	assert.Equal(t, "yaml-pets-service", cfgFromEnvFile.ServiceName)
	assert.Equal(t, 45.5, cfgFromEnvFile.SamplePercentage)
}

func TestInit_Sampling(t *testing.T) {
	ctx := context.Background()

	t.Run("0% sampling drops root spans but respects remote parent", func(t *testing.T) {
		cfg := Config{
			ServiceName:      "test-sampler-0",
			ServiceVersion:   "1.0.0",
			ExporterType:     "none",
			SamplePercentage: 0.0,
		}
		shutdown, err := Init(ctx, cfg)
		require.NoError(t, err)
		defer func() { _ = shutdown(ctx) }()

		tracer := otel.Tracer("test")

		// Root span should NOT be sampled with 0%
		_, rootSpan := tracer.Start(ctx, "root-span")
		assert.False(t, rootSpan.SpanContext().IsSampled())
		rootSpan.End()

		// Remote sampled parent should STILL be sampled (ParentBased)
		remoteSpanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
			TraceFlags: trace.FlagsSampled,
			Remote:     true,
		})
		parentCtx := trace.ContextWithRemoteSpanContext(ctx, remoteSpanContext)
		_, childSpan := tracer.Start(parentCtx, "child-span")
		assert.True(t, childSpan.SpanContext().IsSampled(), "expected ParentBased to respect remote sampled parent")
		childSpan.End()
	})

	t.Run("100% sampling samples root spans", func(t *testing.T) {
		cfg := Config{
			ServiceName:      "test-sampler-100",
			ServiceVersion:   "1.0.0",
			ExporterType:     "none",
			SamplePercentage: 100.0,
		}
		shutdown, err := Init(ctx, cfg)
		require.NoError(t, err)
		defer func() { _ = shutdown(ctx) }()

		tracer := otel.Tracer("test")
		_, rootSpan := tracer.Start(ctx, "root-span")
		assert.True(t, rootSpan.SpanContext().IsSampled())
		rootSpan.End()
	})
}

func TestInit(t *testing.T) {
	ctx := context.Background()

	t.Run("none exporter", func(t *testing.T) {
		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			ExporterType:   "none",
		}
		shutdown, err := Init(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, shutdown)
		err = shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("stdout exporter", func(t *testing.T) {
		cfg := Config{
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			ExporterType:   "stdout",
		}
		shutdown, err := Init(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, shutdown)
		err = shutdown(ctx)
		require.NoError(t, err)
	})
}

func TestInitAndConnectInterceptor(t *testing.T) {
	ctx := context.Background()

	// Use an in-memory span recorder to verify trace extraction
	spanRecorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	interceptor, err := NewConnectInterceptor()
	require.NoError(t, err)
	require.NotNil(t, interceptor)

	// Create a dummy service to test interceptor with incoming W3C traceparent
	dummySvc := &mockPetService{}
	_, handler := petv1connect.NewPetServiceHandler(
		dummySvc,
		connect.WithInterceptors(interceptor),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	client := petv1connect.NewPetServiceClient(server.Client(), server.URL)

	// Simulate frontend-created W3C traceparent header
	frontendTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	frontendSpanID := "00f067aa0ba902b7"
	traceparent := "00-" + frontendTraceID + "-" + frontendSpanID + "-01"

	req := connect.NewRequest(&petv1.GetPetRequest{Id: "test-id"})
	req.Header().Set("traceparent", traceparent)

	_, err = client.GetPet(ctx, req)
	require.NoError(t, err)

	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	// Verify that the backend span adopted the frontend trace ID
	var foundAdoptedTrace bool
	for _, span := range spans {
		if span.SpanContext().TraceID().String() == frontendTraceID {
			foundAdoptedTrace = true
			assert.Equal(t, frontendSpanID, span.Parent().SpanID().String())
			break
		}
	}

	assert.True(t, foundAdoptedTrace, "expected backend span to adopt frontend trace ID %s", frontendTraceID)
}

// mockPetService implements petv1connect.PetServiceHandler for testing
type mockPetService struct {
	petv1connect.UnimplementedPetServiceHandler
}

func (m *mockPetService) GetPet(ctx context.Context, req *connect.Request[petv1.GetPetRequest]) (*connect.Response[petv1.GetPetResponse], error) {
	// Span should be active in context
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("no valid span in context"))
	}
	return connect.NewResponse(&petv1.GetPetResponse{
		Pet: &petv1.Pet{Id: req.Msg.Id, Name: "Fido"},
	}), nil
}
