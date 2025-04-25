package broker

import (
	"context"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/segmentio/kafka-go"
)

// Message represents a generic message obtained from the broker.
// It encapsulates the message data and provides methods for lifecycle management (Ack, Retry, MoveToDLQ).
type Message interface {
	// Data returns the unmarshalled message data.
	Data() domain.Notification
	// GetRetryCount returns the current retry attempt number for the message.
	GetRetryCount() int
	// Ack acknowledges the successful processing of the message.
	Ack(ctx context.Context) error
	// Retry signals that the message processing failed and should be attempted again.
	// The implementation handles updating metadata (like retry count) and rescheduling with a delay.
	Retry(ctx context.Context, delay time.Duration) error
	// MoveToDLQ moves the message to the Dead Letter Queue, typically after exhausting retries.
	MoveToDLQ(ctx context.Context, processingError error) error
	// Headers returns the message headers (e.g., for trace propagation).
	Headers() []kafka.Header
}

// ChannelHandler defines the function signature for the core business logic processing a message.
type ChannelHandler func(ctx context.Context, data domain.Notification) error

// MessageBroker defines the interface for interacting with a message broker infrastructure.
type MessageBroker interface {
	// Consume starts consuming messages from the configured source (e.g., topic/queue).
	// It fetches messages, potentially unmarshals them, wraps them in the Message[T] interface,
	// and passes each message to the provided consumeFunc for processing.
	// The implementation of Consume should handle the consumption loop and error recovery related
	// to fetching messages from the broker itself. Errors returned by consumeFunc should be handled
	// by the logic within consumeFunc (e.g., calling msg.Retry or msg.MoveToDLQ).
	Consume(ctx context.Context, consumeFunc func(ctx context.Context, msg Message) error) error

	// Close terminates the connection to the message broker and cleans up resources.
	Close() error

	// Publish allows publishing a message. While not strictly required by the consumer use case,
	// it might be needed internally by the broker implementation (e.g., for Retry/DLQ).
	// Let's keep it commented out for now unless explicitly needed by the implementation.
	// Publish(ctx context.Context, key []byte, value T, headers map[string]string) error
}
