package notification

type DispatchNotificationInput struct {
	Body        string `json:"body"`
	Destination string `json:"destination"`
}
