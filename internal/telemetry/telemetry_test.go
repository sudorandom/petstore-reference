package telemetry

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
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

	cfg := LoadConfigFromEnv()
	if cfg.ServiceName != "pets-service" {
		t.Errorf("expected default service name pets-service, got %s", cfg.ServiceName)
	}
	if cfg.ExporterType != "none" {
		t.Errorf("expected default exporter none, got %s", cfg.ExporterType)
	}

	// Test custom env
	_ = os.Setenv("OTEL_SERVICE_NAME", "custom-petstore")
	_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	defer func() {
		os.Unsetenv("OTEL_SERVICE_NAME")
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}()

	cfg = LoadConfigFromEnv()
	if cfg.ServiceName != "custom-petstore" {
		t.Errorf("expected custom service name custom-petstore, got %s", cfg.ServiceName)
	}
	if cfg.ExporterType != "otlp" {
		t.Errorf("expected exporter otlp, got %s", cfg.ExporterType)
	}
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
	if err != nil {
		t.Fatalf("failed to create connect interceptor: %v", err)
	}
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}

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
	if err != nil {
		t.Fatalf("GetPet RPC failed: %v", err)
	}

	spans := spanRecorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span recorded")
	}

	// Verify that the backend span adopted the frontend trace ID
	var foundAdoptedTrace bool
	for _, span := range spans {
		if span.SpanContext().TraceID().String() == frontendTraceID {
			foundAdoptedTrace = true
			if span.Parent().SpanID().String() != frontendSpanID {
				t.Errorf("expected parent span ID %s, got %s", frontendSpanID, span.Parent().SpanID().String())
			}
			break
		}
	}

	if !foundAdoptedTrace {
		t.Errorf("expected backend span to adopt frontend trace ID %s, recorded spans: %+v", frontendTraceID, spans)
	}
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
