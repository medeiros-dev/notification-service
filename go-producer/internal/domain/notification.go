package domain

import "time"

type Notification struct {
	ID          string      `json:"id"`
	Content     string      `json:"content"`
	ChannelType string      `json:"channel_type"`
	Destination string      `json:"destination"`
	UserProfile UserProfile `json:"user_profile"`
	CreatedAt   time.Time   `json:"created_at"`
}

type UserProfile struct {
	UserID            string `json:"user_id"`
	Name              string `json:"name"`
	PreferredLanguage string `json:"preferred_language"`
	Timezone          string `json:"timezone"`
}
