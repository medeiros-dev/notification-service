package notification

import "github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"

func NewDispatchNotification(channel channel.Channel) *DispatchNotificationHandler {
	usecase := NewDispatchNotificationUseCase(channel)
	handler := NewDispatchNotificationHandler(usecase)
	return handler
}
