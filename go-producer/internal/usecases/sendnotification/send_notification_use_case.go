package sendnotification

import (
	"context"
	"slices"

	"github.com/google/uuid" // Using google/uuid for generating IDs
	"github.com/medeiros-dev/notification-service/go-producer/configs"
	"github.com/medeiros-dev/notification-service/go-producer/internal/domain"
	"github.com/medeiros-dev/notification-service/go-producer/internal/domain/port/broker"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
)

// SendNotificationUseCase defines the contract for the send notification use case.
type SendNotificationUseCase interface {
	Execute(ctx context.Context, input SendNotificationInputDTO) (SendNotificationOutputDTO, error)
}

// sendNotificationUseCase implements the SendNotificationUseCase interface.
type sendNotificationUseCase struct {
	// messageBroker is the broker that will be used to send the notification.
	messageBroker broker.MessageBroker
}

// NewSendNotificationUseCase creates a new instance of sendNotificationUseCase.
// It requires a messageBroker to be provided.
func NewSendNotificationUseCase(messageBroker broker.MessageBroker) SendNotificationUseCase {
	return &sendNotificationUseCase{
		messageBroker: messageBroker,
	}
}

func (s *sendNotificationUseCase) Execute(ctx context.Context, input SendNotificationInputDTO) (SendNotificationOutputDTO, error) {
	// Start a manual span for the use case
	ctx, span := tracing.Tracer.Start(ctx, "SendNotificationUseCase.Execute")
	defer span.End()

	notifications := []MessageResponse{}
	for _, channel := range input.Channels {
		if !slices.Contains(configs.GetConfig().EnabledChannels, channel.Type) {
			continue
		}
		if !channel.Enabled {
			continue
		}
		metrics.MessagesReceivedTotal.WithLabelValues(channel.Type).Inc()
		notificationMessage := domain.Notification{
			ID: uuid.New().String(),
			UserProfile: domain.UserProfile{
				UserID: input.UserProfile.UserID,
			},
			Content:     input.Content,
			ChannelType: channel.Type,
			Destination: channel.Destination,
		}
		err := s.messageBroker.SendWithContext(ctx, notificationMessage)
		if err != nil {
			notifications = append(notifications, MessageResponse{
				ID:          notificationMessage.ID,
				ChannelType: notificationMessage.ChannelType,
				Status:      "error",
				Error:       err.Error(),
			})
			continue
		}
		notifications = append(notifications, MessageResponse{
			ID:          notificationMessage.ID,
			ChannelType: notificationMessage.ChannelType,
			Status:      "success",
		})
	}
	return SendNotificationOutputDTO{
		Messages: notifications,
	}, nil
}
