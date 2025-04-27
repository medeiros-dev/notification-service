package sendnotification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
	"github.com/medeiros-dev/notification-service/go-producer/pkg/logger"
	"go.uber.org/zap"
)

type SendNotificationHandler struct {
	useCase SendNotificationUseCase
}

func NewSendNotificationHandler(useCase SendNotificationUseCase) *SendNotificationHandler {
	return &SendNotificationHandler{
		useCase: useCase,
	}
}

func (h *SendNotificationHandler) Handle(c *gin.Context) {
	var input SendNotificationInputDTO

	ctx, span := tracing.Tracer.Start(c.Request.Context(), "SendNotificationHandler.Handle")
	defer span.End()

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	channelTypes := make([]string, 0, len(input.Channels))
	for _, ch := range input.Channels {
		channelTypes = append(channelTypes, ch.Type)
	}

	output, err := h.useCase.Execute(ctx, input)
	if err != nil {
		logger.L().Error("Error sending notification via use case",
			zap.String("notificationID", input.ID),
			zap.Strings("channelTypes", channelTypes),
			zap.String("traceID", logger.TraceIDFromContext(ctx)),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process notification"})
		return
	}

	c.JSON(http.StatusOK, output)
}
