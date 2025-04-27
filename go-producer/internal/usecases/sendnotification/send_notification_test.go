package sendnotification

import (
	"context"
	"testing"

	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace/noop"
)

// MockSendNotificationUseCase is a mock implementation of SendNotificationUseCase interface.
type MockSendNotificationUseCase struct {
	mock.Mock
}

// Execute mocks the Execute method.
func (m *MockSendNotificationUseCase) Execute(ctx context.Context, input SendNotificationInputDTO) (SendNotificationOutputDTO, error) {
	args := m.Called(ctx, input)
	// Need to be careful with type assertion for the first return value
	output, _ := args.Get(0).(SendNotificationOutputDTO)
	return output, args.Error(1)
}

func TestSendNotificationUseCase_Execute(t *testing.T) {
	// --- Test Setup ---
	// Init Metrics (using default registry - potential parallel issues)
	// We need to initialize metrics to avoid nil pointer dereference when Inc() is called.
	// Using a new registry per test might be safer for parallel execution.
	metrics.InitMetrics()

	// Init Tracer (using NoOp tracer for tests)
	originalTracer := tracing.Tracer                                        // Store original
	tracing.Tracer = noop.NewTracerProvider().Tracer("test-usecase-tracer") // Use correct non-deprecated provider
	defer func() { tracing.Tracer = originalTracer }()                      // Restore original tracer

	// Initialize a default config for testing, bypassing file loading.
}
