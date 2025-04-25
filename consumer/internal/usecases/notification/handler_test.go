package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	// Alias the package to avoid collision with the test package name
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDispatchNotificationUseCase mocks the use case interface.
type MockDispatchNotificationUseCase struct {
	mock.Mock
	// Ensure it implements the interface
	_ DispatchNotificationUseCase
}

// Execute mocks the Execute method.
func (m *MockDispatchNotificationUseCase) Execute(ctx context.Context, input DispatchNotificationInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

// --- Handler Tests ---

func TestDispatchNotificationHandler_Handle(t *testing.T) {
	baseCtx := context.Background()
	notification := domain.Notification{
		ID:          "notif-123",
		ChannelType: "email", // Assuming handler logic might use this later
		Content:     "Email Content",
		Destination: "recipient@example.com",
		UserProfile: domain.UserProfile{
			UserID: "user-456",
		},
	}
	// Expected input for the use case
	expectedInput := DispatchNotificationInput{
		Body:        notification.Content,
		Destination: notification.Destination,
	}

	tests := []struct {
		name        string
		ctx         context.Context
		setupMock   func(mockUC *MockDispatchNotificationUseCase)
		inputNotif  domain.Notification
		expectError bool
		expectedErr error // Store the specific expected error for comparison
	}{
		{
			name: "Success Path",
			ctx:  baseCtx,
			setupMock: func(mockUC *MockDispatchNotificationUseCase) {
				mockUC.On("Execute", baseCtx, expectedInput).Return(nil)
			},
			inputNotif:  notification,
			expectError: false,
		},
		{
			name: "Success Path with TraceID and Attempt",
			// Create context with values
			ctx: context.WithValue(context.WithValue(baseCtx, "traceID", "trace-abc"), "attempt", 2),
			setupMock: func(mockUC *MockDispatchNotificationUseCase) {
				// Ensure the mock expects the context *with* the values
				expectingCtx := context.WithValue(context.WithValue(baseCtx, "traceID", "trace-abc"), "attempt", 2)
				mockUC.On("Execute", expectingCtx, expectedInput).Return(nil)
			},
			inputNotif:  notification,
			expectError: false,
		},
		{
			name: "Use Case Error",
			ctx:  baseCtx,
			setupMock: func(mockUC *MockDispatchNotificationUseCase) {
				expectedErr := errors.New("use case failed")
				mockUC.On("Execute", baseCtx, expectedInput).Return(expectedErr)
			},
			inputNotif:  notification,
			expectError: true,
			expectedErr: errors.New("use case failed"),
		},
		// Add more tests: e.g., different notification structures, context cancellation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create the mock use case
			mockUC := new(MockDispatchNotificationUseCase)

			// Arrange: Set up mock expectations
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}

			// Arrange: Create the handler, injecting the mock use case
			// This assumes NewDispatchNotificationHandler now accepts the interface
			handler := NewDispatchNotificationHandler(mockUC)

			// Act: Call the handler method
			err := handler.Handle(tt.ctx, tt.inputNotif)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					// Check if the error message matches if a specific error is expected
					assert.EqualError(t, err, tt.expectedErr.Error())
				}
			} else {
				assert.NoError(t, err)
			}

			// Assert: Verify mock expectations were met
			mockUC.AssertExpectations(t)
		})
	}
}
