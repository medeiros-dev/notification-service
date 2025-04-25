package queueconsumer

import (
	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/broker"
	"github.com/medeiros-dev/notification-service/consumers/internal/interfaces"
	"github.com/medeiros-dev/notification-service/consumers/internal/usecases/notification"
)

func NewQueueConsumer(messageBroker broker.MessageBroker, concreteChannels map[string]*notification.DispatchNotificationHandler, cfg *configs.QueueConsumerConfig) *QueueConsumerHandler {
	semaphore := make(chan struct{}, cfg.WorkerPoolSize)
	// Create a map of the interface type
	interfaceChannels := make(map[string]interfaces.NotificationHandlerInterface, len(concreteChannels))
	for k, v := range concreteChannels {
		interfaceChannels[k] = v // Assign concrete type to interface map
	}
	// Pass the interface map to the use case constructor
	useCase := NewQueueConsumerUseCase(messageBroker, interfaceChannels, cfg.MaxRetries, semaphore)
	handler := NewQueueConsumerHandler(useCase)
	return handler
}
