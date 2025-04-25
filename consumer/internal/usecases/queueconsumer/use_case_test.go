package queueconsumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/broker"
	"github.com/medeiros-dev/notification-service/consumers/internal/interfaces"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockMessage struct {
	mock.Mock
	data       domain.Notification
	retryCount int
	headers    []kafka.Header // for Headers() method
}

func (m *MockMessage) Data() domain.Notification {
	return m.data
}
func (m *MockMessage) GetRetryCount() int {
	return m.retryCount
}
func (m *MockMessage) Ack(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockMessage) Retry(ctx context.Context, delay time.Duration) error {
	args := m.Called(ctx, delay)
	return args.Error(0)
}
func (m *MockMessage) MoveToDLQ(ctx context.Context, processingError error) error {
	args := m.Called(ctx, processingError)
	return args.Error(0)
}
func (m *MockMessage) Headers() []kafka.Header {
	return m.headers
}

// Satisfy the broker.Message interface
var _ broker.Message = (*MockMessage)(nil)

type MockMessageBroker struct {
	mock.Mock
}

func (m *MockMessageBroker) Consume(ctx context.Context, consumeFunc func(ctx context.Context, msg broker.Message) error) error {
	args := m.Called(ctx, consumeFunc)
	return args.Error(0)
}
func (m *MockMessageBroker) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Satisfy the broker.MessageBroker interface
var _ broker.MessageBroker = (*MockMessageBroker)(nil)

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) Handle(ctx context.Context, notification domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

// Satisfy the NotificationHandlerInterface
var _ interfaces.NotificationHandlerInterface = (*MockHandler)(nil)

// --- Test Data ---

func sampleNotification(channelType string) domain.Notification {
	return domain.Notification{
		ID:          "notif-1",
		Content:     "test content",
		ChannelType: channelType,
		Destination: "dest",
		UserProfile: domain.UserProfile{UserID: "user-1"},
		CreatedAt:   time.Now(),
	}
}

// --- Tests ---

func setTestConfig(enabledChannels []string) {
	configs.SetTestConfig(&configs.Config{
		EnabledChannels: enabledChannels,
	})
}

func TestProcessMessage_TableDriven(t *testing.T) {
	testCases := []struct {
		name            string
		enabledChannels []string
		notification    domain.Notification
		handlerBehavior func(handler *MockHandler, notif domain.Notification)
		msgSetup        func(msg *MockMessage)
		channels        map[string]interfaces.NotificationHandlerInterface
		assertions      func(t *testing.T, msg *MockMessage, handler *MockHandler)
	}{
		{
			name:            "Success",
			enabledChannels: []string{"email"},
			notification:    sampleNotification("email"),
			handlerBehavior: func(handler *MockHandler, notif domain.Notification) {
				handler.On("Handle", mock.Anything, notif).Return(nil)
			},
			msgSetup: func(msg *MockMessage) {
				msg.On("Ack", mock.Anything).Return(nil)
				msg.On("MoveToDLQ", mock.Anything, mock.Anything).Return(nil)
			},
			channels: func() map[string]interfaces.NotificationHandlerInterface {
				h := new(MockHandler)
				return map[string]interfaces.NotificationHandlerInterface{"email": h}
			}(),
			assertions: func(t *testing.T, msg *MockMessage, handler *MockHandler) {
				msg.AssertCalled(t, "Ack", mock.Anything)
				handler.AssertCalled(t, "Handle", mock.Anything, msg.data)
			},
		},
		{
			name:            "Handler Error Triggers Retry",
			enabledChannels: []string{"email"},
			notification:    sampleNotification("email"),
			handlerBehavior: func(handler *MockHandler, notif domain.Notification) {
				handler.On("Handle", mock.Anything, notif).Return(errors.New("handler failed"))
			},
			msgSetup: func(msg *MockMessage) {
				msg.On("Ack", mock.Anything).Return(nil)
				msg.On("Retry", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)
				msg.On("MoveToDLQ", mock.Anything, mock.Anything).Return(nil)
			},
			channels: func() map[string]interfaces.NotificationHandlerInterface {
				h := new(MockHandler)
				return map[string]interfaces.NotificationHandlerInterface{"email": h}
			}(),
			assertions: func(t *testing.T, msg *MockMessage, handler *MockHandler) {
				handler.AssertCalled(t, "Handle", mock.Anything, msg.data)
				msg.AssertCalled(t, "Retry", mock.Anything, mock.AnythingOfType("time.Duration"))
			},
		},
		{
			name:            "Validation Failure Channel Not Enabled",
			enabledChannels: []string{},
			notification:    sampleNotification("sms"),
			handlerBehavior: func(handler *MockHandler, notif domain.Notification) {}, // no handler
			msgSetup: func(msg *MockMessage) {
				msg.On("MoveToDLQ", mock.Anything, mock.Anything).Return(nil)
			},
			channels: map[string]interfaces.NotificationHandlerInterface{},
			assertions: func(t *testing.T, msg *MockMessage, handler *MockHandler) {
				msg.AssertCalled(t, "MoveToDLQ", mock.Anything, mock.Anything)
			},
		},
		{
			name:            "Panic Recovery Moves To DLQ",
			enabledChannels: []string{"email"},
			notification:    sampleNotification("email"),
			handlerBehavior: func(handler *MockHandler, notif domain.Notification) {
				handler.On("Handle", mock.Anything, notif).Run(func(args mock.Arguments) {
					panic("simulated panic")
				}).Return(nil)
			},
			msgSetup: func(msg *MockMessage) {
				msg.On("MoveToDLQ", mock.Anything, mock.Anything).Return(nil)
			},
			channels: func() map[string]interfaces.NotificationHandlerInterface {
				h := new(MockHandler)
				return map[string]interfaces.NotificationHandlerInterface{"email": h}
			}(),
			assertions: func(t *testing.T, msg *MockMessage, handler *MockHandler) {
				msg.AssertCalled(t, "MoveToDLQ", mock.Anything, mock.Anything)
			},
		},
		{
			name:            "Ack Error Logs Error",
			enabledChannels: []string{"email"},
			notification:    sampleNotification("email"),
			handlerBehavior: func(handler *MockHandler, notif domain.Notification) {
				handler.On("Handle", mock.Anything, notif).Return(nil)
			},
			msgSetup: func(msg *MockMessage) {
				msg.On("Ack", mock.Anything).Return(errors.New("ack failed"))
				msg.On("MoveToDLQ", mock.Anything, mock.Anything).Return(nil)
			},
			channels: func() map[string]interfaces.NotificationHandlerInterface {
				h := new(MockHandler)
				return map[string]interfaces.NotificationHandlerInterface{"email": h}
			}(),
			assertions: func(t *testing.T, msg *MockMessage, handler *MockHandler) {
				msg.AssertCalled(t, "Ack", mock.Anything)
				handler.AssertCalled(t, "Handle", mock.Anything, msg.data)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setTestConfig(tc.enabledChannels)
			ctx := context.Background()

			mockMsg := new(MockMessage)
			mockMsg.data = tc.notification
			mockMsg.retryCount = 0
			if tc.msgSetup != nil {
				tc.msgSetup(mockMsg)
			}

			var mockHandler *MockHandler
			if len(tc.channels) > 0 {
				for _, h := range tc.channels {
					mockHandler = h.(*MockHandler)
					break
				}
				if tc.handlerBehavior != nil {
					tc.handlerBehavior(mockHandler, tc.notification)
				}
			}

			uc := &QueueConsumerUseCase{
				messageBroker: nil,
				channels:      tc.channels,
				maxRetries:    3,
				semaphore:     make(chan struct{}, 1),
			}

			uc.processMessage(ctx, mockMsg)

			if tc.assertions != nil {
				tc.assertions(t, mockMsg, mockHandler)
			}
		})
	}
}
