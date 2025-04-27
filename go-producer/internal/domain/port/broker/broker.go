package broker

import (
	"context"

	"github.com/medeiros-dev/notification-service/go-producer/internal/domain"
)

type MessageBroker interface {
	SendWithContext(ctx context.Context, notification domain.Notification) error
}
