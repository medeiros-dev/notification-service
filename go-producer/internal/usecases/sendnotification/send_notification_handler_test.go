package sendnotification

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/medeiros-dev/notification-service/go-producer/internal/domain"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace/noop"
)

// Helper function to create a test Gin context and recorder
func setupTestRouter() (*gin.Engine, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.Default()
	return r, w
}

func TestSendNotificationHandler_Handle(t *testing.T) {
	// --- Test Setup ---
	originalTracer := tracing.Tracer
	tracing.Tracer = noop.NewTracerProvider().Tracer("test-handler-tracer")
	defer func() { tracing.Tracer = originalTracer }() // Restore original tracer

	// Common setup
	validInput := SendNotificationInputDTO{
		UserProfile: domain.UserProfile{UserID: "user-test"},
		Content:     "Handler test",
		Channels: []Channel{
			{Type: "email", Destination: "handler@test.com", Enabled: true},
		},
	}
	validInputJSON, _ := json.Marshal(validInput)

	expectedSuccessOutput := SendNotificationOutputDTO{
		Messages: []MessageResponse{
			{ID: "msg-1", ChannelType: "email", Status: "success"},
		},
	}

	tests := []struct {
		name               string
		body               []byte
		mockUseCaseSetup   func(*MockSendNotificationUseCase)
		expectedStatusCode int
		expectedBody       string // Use string contains matching for simplicity
	}{
		{
			name: "Success Case",
			body: validInputJSON,
			mockUseCaseSetup: func(muc *MockSendNotificationUseCase) {
				muc.On("Execute", mock.Anything, validInput).Return(expectedSuccessOutput, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       `"status":"success"`, // Check if success message is present
		},
		{
			name:               "Bad Request - Invalid JSON",
			body:               []byte(`{invalid json`), // Malformed JSON
			mockUseCaseSetup:   nil,                     // Use case should not be called
			expectedStatusCode: http.StatusBadRequest,
			expectedBody:       `"error":"Invalid request payload`, // Check for binding error
		},
		{
			name: "Internal Server Error - Use Case Fails",
			body: validInputJSON,
			mockUseCaseSetup: func(muc *MockSendNotificationUseCase) {
				muc.On("Execute", mock.Anything, validInput).Return(SendNotificationOutputDTO{}, errors.New("use case error")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedBody:       `"error":"Failed to process notification"`, // Check for generic server error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockUseCase := new(MockSendNotificationUseCase)
			if tt.mockUseCaseSetup != nil {
				tt.mockUseCaseSetup(mockUseCase)
			}

			// Use the constructor to create the handler with the mock
			handler := NewSendNotificationHandler(mockUseCase)

			router, w := setupTestRouter()
			router.POST("/test", handler.Handle) // Register handler to a test route

			req, _ := http.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(tt.body))
			req.Header.Set("Content-Type", "application/json")

			// Act
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
			mockUseCase.AssertExpectations(t) // Verify mock calls
		})
	}
}
