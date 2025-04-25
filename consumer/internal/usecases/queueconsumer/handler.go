package queueconsumer

import "context"

type QueueConsumerHandler struct {
	queueConsumerUseCase *QueueConsumerUseCase
}

func NewQueueConsumerHandler(queueConsumerUseCase *QueueConsumerUseCase) *QueueConsumerHandler {
	return &QueueConsumerHandler{queueConsumerUseCase: queueConsumerUseCase}
}

func (h *QueueConsumerHandler) Handle(ctx context.Context) error {
	return h.queueConsumerUseCase.Execute(ctx)
}
