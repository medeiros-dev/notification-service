package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

// KafkaMessage wraps a kafka-go message and implements the broker.Message interface.
type KafkaMessage struct {
	broker       *KafkaBroker        // Reference to the parent broker for DLQ/Commit/Retry
	kafkaMsg     kafka.Message       // The raw message from kafka-go
	unmarshalled domain.Notification // The unmarshalled application data
}

// Data returns the unmarshalled Notification data.
func (m *KafkaMessage) Data() domain.Notification {
	return m.unmarshalled
}

// Headers returns the raw Kafka headers.
func (m *KafkaMessage) Headers() []kafka.Header {
	return m.kafkaMsg.Headers
}

// GetRetryCount extracts the retry count from the message headers using the helper in broker.go.
func (m *KafkaMessage) GetRetryCount() int {
	return getRetryCount(m.kafkaMsg.Headers) // Assumes getRetryCount is in broker.go
}

// Ack commits the offset for the current message.
func (m *KafkaMessage) Ack(ctx context.Context) error {
	traceID := logger.TraceIDFromContext(ctx) // Extract TraceID for logging
	logger.L().Debug("Acknowledging Kafka message (committing offset)",
		zap.Int64("offset", m.kafkaMsg.Offset),
		zap.String("topic", m.kafkaMsg.Topic),
		zap.String("notificationID", m.Data().ID),
		zap.String("traceID", traceID),
	)
	err := m.broker.reader.CommitMessages(ctx, m.kafkaMsg)
	if err != nil {
		logger.L().Error("Failed to commit Kafka message offset",
			zap.Int64("offset", m.kafkaMsg.Offset),
			zap.String("topic", m.kafkaMsg.Topic),
			zap.String("notificationID", m.Data().ID),
			zap.String("traceID", traceID),
			zap.Error(err),
		)
	}
	return err
}

// Retry publishes the message back to the main topic with an incremented retry count.
// Note: This implementation republishes immediately. The delay parameter is currently ignored.
func (m *KafkaMessage) Retry(ctx context.Context, delay time.Duration) error {
	traceID := logger.TraceIDFromContext(ctx) // Extract TraceID for logging
	currentRetryCount := m.GetRetryCount()
	nextRetryCount := currentRetryCount + 1
	channelType := m.Data().ChannelType
	if channelType == "" {
		channelType = "unknown"
	}

	logger.L().Info("Preparing message for retry",
		zap.Int64("offset", m.kafkaMsg.Offset),
		zap.String("topic", m.kafkaMsg.Topic),
		zap.String("notificationID", m.Data().ID),
		zap.Int("nextRetryCount", nextRetryCount),
		zap.Duration("requestedDelay", delay),
		zap.String("traceID", traceID),
	)

	newHeaders := updateRetryHeader(m.kafkaMsg.Headers, nextRetryCount) // Assumes updateRetryHeader is in broker.go

	propagator := propagation.TraceContext{}
	// otelHeaderCarrier should be accessible as it's defined in broker.go (same package)
	traceCarrier := otelHeaderCarrier{headers: &newHeaders}
	propagator.Inject(ctx, traceCarrier)

	retryMsg := kafka.Message{
		Topic:   m.kafkaMsg.Topic, // Retrying on the original topic
		Key:     m.kafkaMsg.Key,
		Value:   m.kafkaMsg.Value,
		Headers: newHeaders,
		Time:    time.Now(), // Update time for retry message
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := m.broker.writer.WriteMessages(ctxTimeout, retryMsg)
	if err != nil {
		logger.L().Error("Failed to publish retry message",
			zap.String("notificationID", m.Data().ID),
			zap.String("traceID", traceID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish retry message: %w", err)
	}

	// Increment retry counter before acking original message
	metrics.MessagesRetried.WithLabelValues(channelType).Inc()

	ackErr := m.Ack(ctx)
	if ackErr != nil {
		logger.L().Error("Failed to Ack original message after publishing retry",
			zap.Int64("originalOffset", m.kafkaMsg.Offset),
			zap.String("notificationID", m.Data().ID),
			zap.String("traceID", traceID),
			zap.Error(ackErr),
		)
		return fmt.Errorf("failed to ack original message after retry: %w", ackErr)
	}

	logger.L().Info("Retry message published and original message acknowledged",
		zap.String("notificationID", m.Data().ID),
		zap.Int("nextRetryCount", nextRetryCount),
		zap.String("traceID", traceID),
	)
	return nil
}

// MoveToDLQ publishes the message to the configured DLQ topic.
func (m *KafkaMessage) MoveToDLQ(ctx context.Context, processingError error) error {
	traceID := logger.TraceIDFromContext(ctx) // Extract TraceID for logging
	channelType := m.Data().ChannelType
	if channelType == "" {
		channelType = "unknown"
	}
	currentAttempt := m.GetRetryCount()

	if m.broker.dlqTopic == "" {
		logger.L().Warn("DLQ topic not configured. Discarding message.",
			zap.String("notificationID", m.Data().ID),
			zap.Error(processingError),
			zap.String("traceID", traceID),
		)
		// Increment DLQ counter even if discarded
		metrics.MessagesDLQ.WithLabelValues(channelType).Inc()
		return m.Ack(ctx) // Ack to remove from original topic
	}

	logger.L().Warn("Moving message to DLQ",
		zap.String("notificationID", m.Data().ID),
		zap.String("dlqTopic", m.broker.dlqTopic),
		zap.Int("attempt", currentAttempt),
		zap.String("traceID", traceID),
		zap.Error(processingError),
	)

	dlqHeaders := make([]kafka.Header, 0, len(m.kafkaMsg.Headers)+2)
	// Correctly copy existing headers first
	dlqHeaders = append(dlqHeaders, m.kafkaMsg.Headers...)
	dlqHeaders = append(dlqHeaders, kafka.Header{Key: dlqReasonHeader, Value: []byte(processingError.Error())}) // Uses constant from broker.go
	dlqHeaders = updateRetryHeader(dlqHeaders, currentAttempt)                                                  // Add final attempt count, uses helper from broker.go

	propagator := propagation.TraceContext{}
	// otelHeaderCarrier should be accessible as it's defined in broker.go (same package)
	traceCarrier := otelHeaderCarrier{headers: &dlqHeaders}
	propagator.Inject(ctx, traceCarrier)

	dlqMsg := kafka.Message{
		Topic:   m.broker.dlqTopic,
		Key:     m.kafkaMsg.Key,
		Value:   m.kafkaMsg.Value,
		Headers: dlqHeaders,
		Time:    time.Now(),
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := m.broker.writer.WriteMessages(ctxTimeout, dlqMsg)
	if err != nil {
		logger.L().Error("Failed to publish message to DLQ",
			zap.String("notificationID", m.Data().ID),
			zap.String("dlqTopic", m.broker.dlqTopic),
			zap.String("traceID", traceID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish message to DLQ: %w", err)
	}

	// Increment DLQ counter before acking original message
	metrics.MessagesDLQ.WithLabelValues(channelType).Inc()

	ackErr := m.Ack(ctx)
	if ackErr != nil {
		logger.L().Error("Failed to Ack original message after publishing to DLQ",
			zap.Int64("originalOffset", m.kafkaMsg.Offset),
			zap.String("notificationID", m.Data().ID),
			zap.String("traceID", traceID),
			zap.Error(ackErr),
		)
		return fmt.Errorf("failed to ack original message after DLQ: %w", ackErr)
	}

	logger.L().Info("Message published to DLQ and original message acknowledged",
		zap.String("notificationID", m.Data().ID),
		zap.String("dlqTopic", m.broker.dlqTopic),
		zap.String("traceID", traceID),
	)
	return nil
}
