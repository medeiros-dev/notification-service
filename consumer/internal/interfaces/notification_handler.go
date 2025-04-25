package interfaces

import (
	"context"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
)

// NotificationHandlerInterface defines the contract for handling a notification.
// This allows the QueueConsumerUseCase to be decoupled from the specific handler implementation.
type NotificationHandlerInterface interface {
	Handle(ctx context.Context, notification domain.Notification) error
}
