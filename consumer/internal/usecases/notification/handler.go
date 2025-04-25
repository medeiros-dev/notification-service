package notification

import (
	"context"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"go.uber.org/zap"
)

type DispatchNotificationHandler struct {
	// Depend on the interface, not the concrete type
	dispatchNotificationUseCase DispatchNotificationUseCaseInterface
}

func NewDispatchNotificationHandler(dispatchNotificationUseCase DispatchNotificationUseCaseInterface) *DispatchNotificationHandler {
	return &DispatchNotificationHandler{
		dispatchNotificationUseCase: dispatchNotificationUseCase,
	}
}

func (h *DispatchNotificationHandler) Handle(ctx context.Context, notification domain.Notification) error {
	// Retrieve traceID from context if available
	traceID, _ := ctx.Value("traceID").(string)
	// Retrieve attempt from context if available (might be added by QueueConsumerUseCase)
	attempt, _ := ctx.Value("attempt").(int)

	logger.L().Info("Handling notification dispatch",
		zap.String("notificationID", notification.ID),
		zap.String("channelType", notification.ChannelType),
		zap.String("userID", notification.UserProfile.UserID), // Consider if UserID is PII
		zap.String("traceID", traceID),
		zap.Int("attempt", attempt),
	)

	// Prepare input for the use case
	// Assuming the notification.Content contains the email body/details needed.
	// If you need recipient email address, it should ideally be part of the
	// domain.Notification struct or fetched based on notification.UserId.
	// For this example, let's assume Content *is* the email (simplification).
	useCaseInput := DispatchNotificationInput{
		Body:        notification.Content, // Adjust this based on your actual Notification structure and needs
		Destination: notification.Destination,
	}

	// Execute the use case
	err := h.dispatchNotificationUseCase.Execute(ctx, useCaseInput)
	if err != nil {
		logger.L().Error("Failed to dispatch notification via use case",
			zap.String("notificationID", notification.ID),
			zap.String("channelType", notification.ChannelType),
			zap.String("traceID", traceID),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		return err // Return the error to indicate failure to the caller (e.g., QueueConsumerUseCase)
	}

	logger.L().Info("Successfully dispatched notification via use case",
		zap.String("notificationID", notification.ID),
		zap.String("channelType", notification.ChannelType),
		zap.String("traceID", traceID),
		zap.Int("attempt", attempt),
	)
	return nil
}
