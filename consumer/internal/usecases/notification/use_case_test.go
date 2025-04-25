package notification

import (
	"context"
	"errors"
	"testing"

	port "github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

// MockChannel is a mock implementation of the channel.Channel interface
type MockChannel struct {
	mock.Mock
	// Ensure it implements the interface
	_ port.Channel
}

// Send mocks the Send method
func (m *MockChannel) Send(ctx context.Context, body, to string) error {
	args := m.Called(ctx, body, to)
	return args.Error(0)
}

// --- Tests ---

func TestDispatchNotificationUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	defaultInput := DispatchNotificationInput{
		Body:        "Test Body",
		Destination: "test@example.com",
	}

	tests := []struct {
		name          string
		input         DispatchNotificationInput
		setupMock     func(mockCh *MockChannel, input DispatchNotificationInput)
		expectedError error
	}{
		{
			name:  "Success",
			input: defaultInput,
			setupMock: func(mockCh *MockChannel, input DispatchNotificationInput) {
				mockCh.On("Send", ctx, input.Body, input.Destination).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:  "Channel Error",
			input: defaultInput,
			setupMock: func(mockCh *MockChannel, input DispatchNotificationInput) {
				expectedError := errors.New("channel failed")
				mockCh.On("Send", ctx, input.Body, input.Destination).Return(expectedError)
			},
			expectedError: errors.New("channel failed"), // Match the error type
		},
		// Add more cases if needed (e.g., empty body/destination if validation exists)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCh := new(MockChannel)
			useCase := NewDispatchNotificationUseCase(mockCh) // Assumes constructor takes channel.Channel

			// Setup mock expectations
			if tt.setupMock != nil {
				tt.setupMock(mockCh, tt.input)
			}

			// Execute
			err := useCase.Execute(ctx, tt.input)

			// Assert
			if tt.expectedError != nil {
				assert.Error(t, err)
				// Use ErrorContains for more robust error checking if specific messages matter
				// Or assert.Equal if the exact error instance is important
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			mockCh.AssertExpectations(t)
		})
	}
}
