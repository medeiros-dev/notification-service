package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/broker"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

// KafkaBroker implements the broker.MessageBroker interface using Kafka.
type KafkaBroker struct {
	writer     *kafka.Writer
	reader     *kafka.Reader // Single reader for the primary topic
	brokers    []string
	topic      string
	groupID    string
	dlqTopic   string
	maxRetries int
	// mu protects shared resources if needed in the future, e.g., managing multiple readers
	mu sync.Mutex
}

// Config holds configuration for the KafkaBroker.
type Config struct {
	Brokers []string
}

// otelHeaderCarrier adapts kafka-go headers to OpenTelemetry's TextMapCarrier
// This struct and its methods allow extracting/injecting trace context.
type otelHeaderCarrier struct {
	headers *[]kafka.Header
}

func (c otelHeaderCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
func (c otelHeaderCarrier) Set(key, value string) {
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kafka.Header{Key: key, Value: []byte(value)})
}
func (c otelHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}

// NewKafkaBroker creates a new KafkaBroker instance.
func NewKafkaBroker(cfg Config) (*KafkaBroker, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers cannot be empty")
	}

	appConfig := configs.GetConfig()
	topic := appConfig.KafkaTopic
	groupID := appConfig.KafkaGroupID
	maxRetries := appConfig.MaxRetries
	dlqTopic := appConfig.KafkaDLQTopic

	if topic == "" {
		return nil, fmt.Errorf("KAFKA_TOPIC must be set")
	}
	if groupID == "" {
		return nil, fmt.Errorf("KAFKA_GROUP_ID must be set")
	}
	if dlqTopic == "" {
		logger.L().Warn("KAFKA_DLQ_TOPIC is not set. Failed messages exceeding retries will be discarded.")
	}

	// General writer config
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}

	// Reader config
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // Disable auto-commit, we'll commit manually
	})

	logger.L().Info("Kafka Broker initialized",
		zap.String("topic", topic),
		zap.String("groupID", groupID),
		zap.Int("maxRetries", maxRetries),
		zap.String("dlqTopic", dlqTopic),
		zap.Strings("brokers", cfg.Brokers),
	)

	return &KafkaBroker{
		writer:     w,
		reader:     r,
		brokers:    cfg.Brokers,
		topic:      topic,
		groupID:    groupID,
		dlqTopic:   dlqTopic,
		maxRetries: maxRetries,
	}, nil
}

// Consume fetches messages from Kafka and passes them to the consumeFunc.
func (kb *KafkaBroker) Consume(
	ctx context.Context,
	consumeFunc func(ctx context.Context, msg broker.Message) error,
) error {
	logger.L().Info("Starting Kafka consumer loop",
		zap.String("topic", kb.topic),
		zap.String("groupID", kb.groupID),
	)

	for {
		message, err := kb.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || err == context.Canceled || err == context.DeadlineExceeded {
				logger.L().Info("Context cancelled or deadline exceeded, stopping consumer loop",
					zap.String("topic", kb.topic),
					zap.String("groupID", kb.groupID),
					zap.Error(err), // Include context error if available
				)
				return nil // Clean exit
			}
			// Log transient fetch errors
			logger.L().Error("Error fetching message from Kafka, continuing loop",
				zap.String("topic", kb.topic),
				zap.String("groupID", kb.groupID),
				zap.Error(err),
			)
			time.Sleep(1 * time.Second) // Avoid tight loop on persistent errors
			continue
		}

		logger.L().Debug("Fetched Kafka message",
			zap.String("topic", message.Topic),
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
			zap.ByteString("key", message.Key),
		)

		var data domain.Notification
		if err := json.Unmarshal(message.Value, &data); err != nil {
			logger.L().Error("Error unmarshalling message, attempting to move to DLQ",
				zap.String("topic", message.Topic),
				zap.Int64("offset", message.Offset),
				// Do not log message.Value by default as it might be large/sensitive
				zap.Error(err),
			)
			// Create a temporary KafkaMessage to use MoveToDLQ for the poison pill
			poisonPillMsg := &KafkaMessage{
				broker:   kb,
				kafkaMsg: message,
				// unmarshalled is zero value, but Data() won't be called in MoveToDLQ in this path
			}
			// Inject traceID into context if possible, though it might not be available here
			dlqCtx := ctx // Potentially enrich context later if trace info is in headers
			if dlqErr := poisonPillMsg.MoveToDLQ(dlqCtx, fmt.Errorf("unmarshalling error: %w", err)); dlqErr != nil {
				logger.L().Error("Failed to move unmarshallable message to DLQ. Message may be reprocessed.",
					zap.Int64("offset", message.Offset),
					zap.String("topic", message.Topic),
					zap.Error(dlqErr),
				)
				// If moving to DLQ fails, we don't commit, Kafka will redeliver.
			}
			continue // Skip processing this message further
		}

		// Increment received message counter
		// Use "unknown" if ChannelType is somehow empty after unmarshalling
		channelType := data.ChannelType
		if channelType == "" {
			channelType = "unknown"
		}
		metrics.MessagesReceived.WithLabelValues(channelType).Inc()

		// Create the message object that implements the interface
		appMsg := &KafkaMessage{
			broker:       kb,
			kafkaMsg:     message,
			unmarshalled: data,
		}

		// TODO: Extract traceID from message headers if present and add to context
		// Extract OpenTelemetry trace context from Kafka headers
		headersCarrier := otelHeaderCarrier{headers: &message.Headers}
		propagator := propagation.TraceContext{}
		processingCtx := propagator.Extract(ctx, headersCarrier)

		// Execute the processing logic provided by the use case
		processingErr := consumeFunc(processingCtx, appMsg)

		// The consumeFunc is responsible for calling Ack, Retry, or MoveToDLQ.
		// We log if the consumeFunc itself returned an error, which might indicate
		// a failure in the Ack/Retry/MoveToDLQ operations, or a panic recovery.
		if processingErr != nil {
			logger.L().Error("Error returned by consumeFunc. Action (Ack/Retry/DLQ) might have failed or panic occurred.",
				zap.Int64("offset", message.Offset),
				zap.String("topic", message.Topic),
				zap.String("notificationID", appMsg.Data().ID),
				zap.Error(processingErr),
			)
			// Depending on the error type or policy, we might want to stop the consumer
			// or just log and continue. For now, log and continue.
			// If Ack/Retry/DLQ failed, the offset wasn't committed, so Kafka will redeliver.
		}

		// Check context after processing each message to allow graceful shutdown
		if ctx.Err() != nil {
			logger.L().Info("Context cancelled during processing, stopping consumer loop",
				zap.String("topic", kb.topic),
				zap.String("groupID", kb.groupID),
			)
			return nil
		}
	}
}

// Close cleans up the Kafka reader and writer.
func (kb *KafkaBroker) Close() error {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	var readerErr, writerErr error

	logger.L().Info("Closing Kafka reader...")
	if kb.reader != nil {
		readerErr = kb.reader.Close()
		if readerErr != nil {
			logger.L().Error("Error closing Kafka reader", zap.Error(readerErr))
		} else {
			logger.L().Info("Kafka reader closed.")
		}
	}

	logger.L().Info("Closing Kafka writer...")
	if kb.writer != nil {
		writerErr = kb.writer.Close()
		if writerErr != nil {
			logger.L().Error("Error closing Kafka writer", zap.Error(writerErr))
		} else {
			logger.L().Info("Kafka writer closed.")
		}
	}

	if readerErr != nil || writerErr != nil {
		// Combine errors for reporting
		combinedErr := fmt.Errorf("error closing Kafka resources (Reader: %v, Writer: %v)", readerErr, writerErr)
		logger.L().Error("Errors occurred during Kafka resource closing", zap.Error(combinedErr))
		return combinedErr
	}
	logger.L().Info("Kafka resources closed successfully.")
	return nil
}
