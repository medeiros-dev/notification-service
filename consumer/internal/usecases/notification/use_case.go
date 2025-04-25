package notification

import (
	"context"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"
)

// DispatchNotificationUseCase defines the interface for the core dispatch logic.
type DispatchNotificationUseCaseInterface interface {
	Execute(ctx context.Context, input DispatchNotificationInput) error
}
type DispatchNotificationUseCase struct {
	channel channel.Channel
}

func NewDispatchNotificationUseCase(channel channel.Channel) *DispatchNotificationUseCase {
	return &DispatchNotificationUseCase{
		channel: channel,
	}
}

func (u *DispatchNotificationUseCase) Execute(ctx context.Context, input DispatchNotificationInput) error {
	return u.channel.Send(ctx, input.Body, input.Destination)
}
