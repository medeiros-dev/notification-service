package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/medeiros-dev/notification-service/go-producer/configs"
	"github.com/medeiros-dev/notification-service/go-producer/internal/domain"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/go-producer/internal/observability/tracing"
	"github.com/medeiros-dev/notification-service/go-producer/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

// KafkaBroker implements the message_broker.MessageBroker interface using Kafka.
type KafkaBroker struct {
	writer        *kafka.Writer
	readerConfigs map[string]*kafka.ReaderConfig // One reader config per topic/channel
	brokers       []string
	mu            sync.Mutex // To protect access to readerConfigs
}

// Config holds configuration for the KafkaBroker.
type Config struct {
	Brokers []string
}

var (
	kafkaBroker *KafkaBroker
)

// otelHeaderCarrier adapts kafka-go headers to OpenTelemetry's TextMapCarrier
// This struct and its methods allow injecting/extracting trace context.
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

	// General writer config - customize as needed
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne, // Or kafka.RequireAll for stronger guarantees
		Async:        false,            // Set to true for higher throughput, handle errors differently
	}

	kafkaBroker = &KafkaBroker{
		writer:        w,
		readerConfigs: make(map[string]*kafka.ReaderConfig),
		brokers:       cfg.Brokers,
	}
	return kafkaBroker, nil
}

func (kb *KafkaBroker) SendWithContext(ctx context.Context, notification domain.Notification) error {
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		metrics.ErrorTotal.WithLabelValues("marshal_json").Inc()
		return fmt.Errorf("failed to marshal notification %s to JSON: %w", notification.ID, err)
	}

	msg := kafka.Message{
		Topic: configs.GetConfig().KafkaTopic,
		Key:   []byte(notification.ID),
		Value: notificationJSON,
	}

	// Inject OpenTelemetry trace context into Kafka headers
	headers := make([]kafka.Header, 0)
	propagator := propagation.TraceContext{}
	ctx, span := tracing.Tracer.Start(ctx, "KafkaProducer.Send")
	defer span.End()
	propagator.Inject(ctx, otelHeaderCarrier{headers: &headers})
	msg.Headers = headers

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	err = kb.writer.WriteMessages(ctxTimeout, msg)
	metrics.KafkaPublishDuration.Observe(time.Since(start).Seconds())
	traceID := logger.TraceIDFromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.L().Error("Failed to write messages to kafka",
			zap.String("notificationID", notification.ID),
			zap.String("channelType", notification.ChannelType),
			zap.String("traceID", traceID),
			zap.Error(err),
		)
		metrics.KafkaPublishTotal.WithLabelValues("failure").Inc()
		metrics.ErrorTotal.WithLabelValues("kafka_publish").Inc()
		metrics.MessagesSentFailedTotal.WithLabelValues(notification.ChannelType).Inc()
		return fmt.Errorf("failed to write messages to kafka: %w", err)
	}

	logger.L().Info("Successfully sent notification via Kafka",
		zap.String("notificationID", notification.ID),
		zap.String("channelType", notification.ChannelType),
		zap.String("traceID", traceID),
	)
	metrics.KafkaPublishTotal.WithLabelValues("success").Inc()
	metrics.MessagesSentSuccessTotal.WithLabelValues(notification.ChannelType).Inc()
	return nil
}

// Close cleans up resources used by the KafkaBroker (e.g., the writer).
// Note: Readers are closed when their consuming goroutine exits.
func (kb *KafkaBroker) Close() error {
	kb.mu.Lock() // Ensure exclusive access for cleanup
	defer kb.mu.Unlock()

	logger.L().Info("Closing Kafka writer...")
	if err := kb.writer.Close(); err != nil {
		logger.L().Error("Failed to close kafka writer", zap.Error(err))
		return fmt.Errorf("failed to close kafka writer: %w", err)
	}
	logger.L().Info("Kafka writer closed.")
	// Readers are closed individually in their goroutines
	kb.readerConfigs = make(map[string]*kafka.ReaderConfig) // Clear reader configs
	return nil
}

func GetKafkaBroker() *KafkaBroker {
	return kafkaBroker
}
