package sendnotification

import "github.com/medeiros-dev/notification-service/go-producer/internal/domain/port/broker"

func NewSendNotification(messageBroker broker.MessageBroker) *SendNotificationHandler {
	useCase := NewSendNotificationUseCase(messageBroker)
	return NewSendNotificationHandler(useCase)
}
