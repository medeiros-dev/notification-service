package domain

import "time"

type Notification struct {
	ID        string    `json:"id"`
	UserId    string    `json:"user_id"`
	Content   string    `json:"content"`
	Channels  []string  `json:"channels"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationChannel interface {
	Send(notification Notification) error

	ChannelName() string
}
