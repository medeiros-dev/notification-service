package sendnotification

import (
	"time"

	"github.com/medeiros-dev/notification-service/go-producer/internal/domain"
)

// SendNotificationInput represents the data needed to send a notification.
type SendNotificationInputDTO struct {
	ID          string             `json:"id"`
	Content     string             `json:"content"`
	Channels    []Channel          `json:"channels"`
	UserProfile domain.UserProfile `json:"user_profile"`
	CreatedAt   time.Time          `json:"created_at"`
}

type Channel struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Enabled     bool   `json:"enabled"`
}

type SendNotificationOutputDTO struct {
	Messages []MessageResponse `json:"messages"`
}

type MessageResponse struct {
	ID          string `json:"id"`
	ChannelType string `json:"channel_type"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}
