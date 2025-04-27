package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/medeiros-dev/notification-service/go-producer/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockExporter is a simple mock implementation of sdktrace.SpanExporter
type mockExporter struct {
	exportErr   error
	shutdownErr error
	spans       []sdktrace.ReadOnlySpan
}

func (m *mockExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if m.exportErr != nil {
		return m.exportErr
	}
	m.spans = append(m.spans, spans...)
	return nil
}

func (m *mockExporter) Shutdown(ctx context.Context) error {
	return m.shutdownErr
}

// Ensure mockExporter implements the interface
var _ sdktrace.SpanExporter = (*mockExporter)(nil)

// Helper to reset global state between tests
func resetGlobals() {
	// Reset the global tracer provider to a no-op one
	otel.SetTracerProvider(noop.NewTracerProvider())
	// Reset the global Tracer variable
	Tracer = nil
	// Reset shutdownFunc to a no-op
	shutdownFunc = func(ctx context.Context) error { return nil }
}

func TestInitTracer_Success_Table(t *testing.T) {
	baseCfg := &configs.Config{
		OtelServiceName: "test-service-table",
		OtelEndpoint:    "fake:4317",
	}

	tests := []struct {
		name         string
		insecure     bool
		expectExport bool // Add flag to control if export is checked
	}{
		{
			name:         "insecure",
			insecure:     true,
			expectExport: true, // Check export on the first run
		},
		{
			name:         "secure",
			insecure:     false,
			expectExport: false, // Don't need to re-check export logic
		},
	}

	// Setup mock exporter once outside the loop
	mockExp := &mockExporter{}
	originalNewExporterFunc := newExporterFunc
	newExporterFunc = func(ctx context.Context, cfg *configs.Config) (sdktrace.SpanExporter, error) {
		return mockExp, nil
	}
	defer func() { newExporterFunc = originalNewExporterFunc }() // Restore original

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetGlobals()
			mockExp.spans = nil // Reset spans for each sub-test if checking export

			cfg := *baseCfg // Copy base config
			cfg.OtelInsecure = tc.insecure

			shutdown, err := InitTracer(&cfg)

			require.NoError(t, err)
			require.NotNil(t, shutdown)
			assert.NotNil(t, Tracer)

			// Verify shutdown calls the mock exporter's shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			err = shutdown(ctx)
			assert.NoError(t, err) // Mock shutdown returns nil error by default

			if tc.expectExport {
				// --- Test span export using a local provider with WithSyncer ---
				// Create a local provider specifically for this export check, using WithSyncer
				res, err := resource.New(context.Background(), resource.WithAttributes(semconv.ServiceName("local-test")))
				require.NoError(t, err, "Failed to create resource for local provider")
				localProvider := sdktrace.NewTracerProvider(
					sdktrace.WithSyncer(mockExp), // Use Syncer for deterministic test
					sdktrace.WithResource(res),
				)
				localTracer := localProvider.Tracer("test-export-check-" + tc.name)

				// Start and end a span using the local tracer
				_, span := localTracer.Start(context.Background(), "test-span-"+tc.name)
				span.End()

				// Shutdown the local provider to ensure export
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer shutdownCancel()
				err = localProvider.Shutdown(shutdownCtx)
				assert.NoError(t, err, "Local provider shutdown failed")

				// Check if the span was exported to the mock exporter (should be synchronous)
				assert.NotEmpty(t, mockExp.spans, "Expected spans to be exported to mock exporter")
				if len(mockExp.spans) > 0 {
					assert.Equal(t, "test-span-"+tc.name, mockExp.spans[0].Name())
				}
			}
		})
	}
}

func TestInitTracer_ExporterError(t *testing.T) {
	resetGlobals()
	// Override the exporter creation to return an error
	expectedErr := errors.New("exporter creation failed")
	originalNewExporterFunc := newExporterFunc
	newExporterFunc = func(ctx context.Context, cfg *configs.Config) (sdktrace.SpanExporter, error) {
		return nil, expectedErr
	}
	defer func() { newExporterFunc = originalNewExporterFunc }() // Restore original

	cfg := &configs.Config{OtelServiceName: "test-service", OtelEndpoint: "fake:4317"}

	shutdown, err := InitTracer(cfg)

	require.Error(t, err)
	assert.Nil(t, shutdown)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, Tracer) // Tracer should not be set on error
}

// Note: Testing resource.New failure is harder without more complex mocking
// or dependency injection for resource.New itself. We skip it for simplicity.

func TestGetTracer(t *testing.T) {
	resetGlobals()
	assert.Nil(t, GetTracer(), "Tracer should be nil initially")

	// Setup mock exporter
	mockExp := &mockExporter{}
	originalNewExporterFunc := newExporterFunc
	newExporterFunc = func(ctx context.Context, cfg *configs.Config) (sdktrace.SpanExporter, error) {
		return mockExp, nil
	}
	defer func() { newExporterFunc = originalNewExporterFunc }()

	cfg := &configs.Config{OtelServiceName: "get-tracer-test"}
	_, err := InitTracer(cfg)
	require.NoError(t, err)

	retrievedTracer := GetTracer()
	assert.NotNil(t, retrievedTracer)
	assert.Equal(t, Tracer, retrievedTracer) // Should return the package global
}

func TestShutdownTracer_Table(t *testing.T) {
	tests := []struct {
		name          string
		shutdownError error // nil for success case
	}{
		{
			name:          "success",
			shutdownError: nil,
		},
		{
			name:          "failure",
			shutdownError: errors.New("shutdown failed"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetGlobals()
			shutdownCalled := false
			var shutdownCtx context.Context

			// Manually set the global shutdownFunc for this test
			shutdownFunc = func(ctx context.Context) error {
				shutdownCalled = true
				shutdownCtx = ctx
				return tc.shutdownError // Use error from test case
			}

			ctx := context.Background()
			ShutdownTracer(ctx)

			assert.True(t, shutdownCalled, "Expected shutdownFunc to be called")
			assert.Equal(t, ctx, shutdownCtx, "Expected context to be passed to shutdownFunc")
			// If tc.shutdownError is not nil, we expect the logger to be called, but don't assert log output directly.
		})
	}
}
