package queueconsumer

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/broker"
	"github.com/medeiros-dev/notification-service/consumers/internal/interfaces"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/metrics"
	"github.com/medeiros-dev/notification-service/consumers/internal/observability/tracing"
	"github.com/medeiros-dev/notification-service/consumers/pkg/backoff"
	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type contextKey string

const (
	DefaultMaxRetries            = 3
	attemptKey        contextKey = "attempt"
)

// --- OTEL Header Carrier (Copied from Producer) ---
// otelHeaderCarrier adapts kafka-go headers to OpenTelemetry's TextMapCarrier
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
	// Set is usually used for injection, but included for completeness
	found := false
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			found = true
			break
		}
	}
	if !found {
		*c.headers = append(*c.headers, kafka.Header{Key: key, Value: []byte(value)})
	}
}
func (c otelHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}

// --- End OTEL Header Carrier ---

type QueueConsumerUseCase struct {
	messageBroker broker.MessageBroker
	channels      map[string]interfaces.NotificationHandlerInterface
	maxRetries    int
	semaphore     chan struct{}
}

func NewQueueConsumerUseCase(
	messageBroker broker.MessageBroker,
	channels map[string]interfaces.NotificationHandlerInterface,
	maxRetries int,
	semaphore chan struct{},
) *QueueConsumerUseCase {
	if maxRetries <= 0 {
		// Use Zap logger for initialization warnings
		logger.L().Warn("Invalid maxRetries provided, defaulting",
			zap.Int("providedMaxRetries", maxRetries),
			zap.Int("defaultMaxRetries", DefaultMaxRetries),
		)
		maxRetries = DefaultMaxRetries
	}
	if len(channels) == 0 {
		logger.L().Warn("No channels provided to QueueConsumerUseCase.")
	}
	return &QueueConsumerUseCase{
		messageBroker: messageBroker,
		channels:      channels,
		maxRetries:    maxRetries,
		semaphore:     semaphore,
	}
}

func (u *QueueConsumerUseCase) Execute(ctx context.Context) error {
	consumeFunc := func(ctx context.Context, msg broker.Message) error {
		// Acquire a semaphore token
		u.semaphore <- struct{}{}

		// --- Extract Trace Context and Start Span ---
		headers := msg.Headers() // Assuming msg has a Headers() method returning []kafka.Header
		carrier := otelHeaderCarrier{headers: &headers}
		propagator := propagation.TraceContext{}
		parentCtx := propagator.Extract(ctx, carrier)

		// Start a new span as a consumer
		consumerCtx, span := tracing.Tracer.Start(parentCtx, "QueueConsumer.processMessage", trace.WithSpanKind(trace.SpanKindConsumer))
		// --- End Trace Extraction ---

		// Launch a goroutine to process the message concurrently
		go func(processingCtx context.Context, message broker.Message) { // Pass new context and original message
			// End the span when the goroutine finishes processing
			defer span.End()

			// Release the semaphore token when the goroutine finishes
			defer func() {
				<-u.semaphore
			}()

			// Remove old traceID extraction
			// traceID := "" // Placeholder
			// processingCtx := context.WithValue(ctx, ctxKeyTraceID, traceID)

			// Delegate processing to the helper method using the new context
			u.processMessage(processingCtx, message)

		}(consumerCtx, msg) // Pass the new consumerCtx and the original msg

		// The Consume function might need adjustments depending on how the message broker
		// handles acknowledgements/retries with asynchronous processing.
		// Returning nil here assumes the broker doesn't need immediate feedback from this function
		// and the goroutine handles the Ack/Retry/DLQ logic.
		return nil
	}

	logger.L().Info("QueueConsumerUseCase starting consumption...")
	return u.messageBroker.Consume(ctx, consumeFunc)
}

// validateMessageAndChannel checks if the channel is enabled and implemented.
// If validation fails, it attempts to move the message to DLQ and returns an error.
// If successful, it returns the channel implementation.
func (u *QueueConsumerUseCase) validateMessageAndChannel(ctx context.Context, msg broker.Message, notification domain.Notification, currentAttempt int) (interfaces.NotificationHandlerInterface, error) {
	// Get traceID from context for logging (using helper)
	traceID := logger.TraceIDFromContext(ctx)
	msgID := notification.ID

	// Check if channel type is enabled in config
	if !slices.Contains(configs.GetConfig().EnabledChannels, notification.ChannelType) {
		errMsg := fmt.Sprintf("channel '%s' is not enabled in configuration", notification.ChannelType)
		logger.L().Warn(errMsg+". Moving message to DLQ.",
			zap.String("notificationID", msgID),
			zap.String("channelType", notification.ChannelType),
			zap.String("traceID", traceID),
		)
		dlqErr := msg.MoveToDLQ(ctx, errors.New(errMsg))
		if dlqErr != nil {
			logger.L().Error("Error moving message to DLQ after disabled channel check",
				zap.String("notificationID", msgID),
				zap.String("channelType", notification.ChannelType),
				zap.String("traceID", traceID),
				zap.Error(dlqErr),
			)
		}
		return nil, errors.New(errMsg)
	}

	// Check if channel implementation exists
	channel, ok := u.channels[notification.ChannelType]
	if !ok {
		errMsg := fmt.Sprintf("no channel implementation found for type '%s'", notification.ChannelType)
		logger.L().Error(errMsg+". Treating as processing error. Moving message to DLQ.",
			zap.String("notificationID", msgID),
			zap.String("channelType", notification.ChannelType),
			zap.Int("attempt", currentAttempt),
			zap.Int("maxRetries", u.maxRetries),
			zap.String("traceID", traceID),
		)
		dlqErr := msg.MoveToDLQ(ctx, errors.New(errMsg))
		if dlqErr != nil {
			logger.L().Error("Error moving message to DLQ after missing channel check",
				zap.String("notificationID", msgID),
				zap.String("channelType", notification.ChannelType),
				zap.String("traceID", traceID),
				zap.Error(dlqErr),
			)
		}
		return nil, errors.New(errMsg)
	}

	return channel, nil
}

// processMessage handles the logic for processing a single message within a goroutine.
func (u *QueueConsumerUseCase) processMessage(ctx context.Context, msg broker.Message) {
	// Get traceID from context for logging (using helper)
	traceID := logger.TraceIDFromContext(ctx)

	// Add recover function for panic safety
	defer func() {
		if r := recover(); r != nil {
			// Log panic with stack trace
			logger.L().Error("CRITICAL: Panic recovered in processMessage",
				zap.Any("panicValue", r),
				zap.String("stacktrace", string(debug.Stack())),
				zap.String("traceID", traceID),
				// Attempt to include message context if possible (be careful with PII/size)
				zap.String("notificationID", msg.Data().ID),
				zap.String("channelType", msg.Data().ChannelType),
			)
			// Decide if message should be moved to DLQ after panic
			// Generally safer to DLQ after unexpected panic to avoid infinite loops
			panicErr := fmt.Errorf("panic recovered: %v", r)
			dlqErr := msg.MoveToDLQ(context.Background(), panicErr) // Use background context for DLQ after panic
			if dlqErr != nil {
				logger.L().Error("Failed to move message to DLQ after panic",
					zap.String("notificationID", msg.Data().ID),
					zap.String("traceID", traceID),
					zap.Error(dlqErr),
				)
			}
		}
	}()

	startTime := time.Now()
	retryCount := msg.GetRetryCount()
	currentAttempt := retryCount + 1
	notification := msg.Data()
	msgID := notification.ID
	channelType := notification.ChannelType
	if channelType == "" {
		channelType = "unknown"
	}

	ctx = context.WithValue(ctx, attemptKey, currentAttempt)

	logger.L().Info("Processing message",
		zap.String("notificationID", msgID),
		zap.String("channelType", channelType),
		zap.Int("attempt", currentAttempt),
		zap.Int("maxRetries", u.maxRetries),
		zap.String("traceID", traceID),
	)

	// Validate channel and get handler
	channelHandler, validationErr := u.validateMessageAndChannel(ctx, msg, notification, currentAttempt)
	if validationErr != nil {
		// Log validation failure explicitly
		logger.L().Warn("Message validation failed",
			zap.String("notificationID", msgID),
			zap.String("channelType", channelType),
			zap.Int("attempt", currentAttempt),
			zap.String("traceID", traceID),
			zap.Error(validationErr),
		)
		// Observe duration even on validation failure (treat as unsuccessful processing)
		metrics.ObserveDuration(channelType, false, startTime) // Use original metric func
		// Assuming MessagesFailed is appropriate for validation errors leading to DLQ
		metrics.MessagesFailed.WithLabelValues(channelType).Inc()
		return // Exit goroutine
	}

	// Execute the channel-specific handler using the interface
	handleErr := channelHandler.Handle(ctx, notification)

	if handleErr != nil {
		// Specific error already logged by channelHandler.Handle or handleSendError
		// Increment failed counter only on the first attempt's failure
		if currentAttempt == 1 {
			metrics.MessagesFailed.WithLabelValues(channelType).Inc() // Use original metric func
		}
		// Observe duration (unsuccessful)
		metrics.ObserveDuration(channelType, false, startTime) // Use original metric func
		// Error handling (retry/DLQ) is delegated.
		u.handleSendError(ctx, msg, notification, handleErr, currentAttempt)
		return // Exit goroutine after handling error/retry
	}

	// Success path
	logger.L().Info("Successfully processed message, acknowledging",
		zap.String("notificationID", msgID),
		zap.String("channelType", channelType),
		zap.Int("attempt", currentAttempt),
		zap.String("traceID", traceID),
	)
	ackErr := msg.Ack(ctx)
	if ackErr != nil {
		// Log acknowledgement error. Depending on broker, this might lead to reprocessing.
		logger.L().Error("Error acknowledging message after successful processing",
			zap.String("notificationID", msgID),
			zap.String("channelType", channelType),
			zap.Int("attempt", currentAttempt), // Keep attempt for context
			zap.String("traceID", traceID),
			zap.Error(ackErr),
		)
		// Observe duration (technically successful processing, but failed ack)
		metrics.ObserveDuration(channelType, true, startTime) // Use original metric func
		// Decide how to handle critical Ack error (e.g., metrics, alerts)
		// Do not increment success counter if Ack fails, as it might be reprocessed.
		return
	}

	// Increment success counter *after* successful Ack
	metrics.MessagesProcessed.WithLabelValues(channelType).Inc() // Use original metric func
	// Observe duration (successful)
	metrics.ObserveDuration(channelType, true, startTime) // Use original metric func
}

// handleSendError handles errors during the channel Send operation (now handler Handle).
func (u *QueueConsumerUseCase) handleSendError(ctx context.Context, msg broker.Message, notification domain.Notification, sendError error, currentAttempt int) {
	// Get traceID from context for logging (using helper)
	traceID := logger.TraceIDFromContext(ctx)
	msgID := notification.ID
	channelType := notification.ChannelType
	if channelType == "" {
		channelType = "unknown"
	}

	// MessagesFailed metric is incremented in processMessage on first attempt failure

	// Log the processing error
	logger.L().Error("Error processing notification", // Keep this log for detailed error context
		zap.String("notificationID", msgID),
		zap.String("channelType", channelType),
		zap.Int("attempt", currentAttempt),
		zap.Int("maxRetries", u.maxRetries),
		zap.String("traceID", traceID),
		zap.Error(sendError),
	)

	// Retry logic
	if currentAttempt < u.maxRetries {
		// Calculate backoff duration using the original function name
		backoffDuration := backoff.CalculateRetryDelayFromConfig(currentAttempt) // Use original backoff func
		logger.L().Info("Scheduling retry",
			zap.String("notificationID", msgID),
			zap.String("channelType", channelType),
			zap.Int("attempt", currentAttempt+1), // Log the *next* attempt number
			zap.Duration("backoffDuration", backoffDuration),
			zap.String("traceID", traceID),
		)

		// Schedule retry using the calculated duration
		retryErr := msg.Retry(ctx, backoffDuration)
		if retryErr != nil {
			logger.L().Error("Failed to schedule retry, moving to DLQ",
				zap.String("notificationID", msgID),
				zap.String("channelType", channelType),
				zap.String("traceID", traceID),
				zap.Error(retryErr), // Log the retry error
				zap.NamedError("originalSendError", sendError),
			)
			metrics.MessagesDLQ.WithLabelValues(channelType).Inc() // Use original metric func
			dlqErr := msg.MoveToDLQ(ctx, fmt.Errorf("failed to schedule retry after processing error: %w; original error: %w", retryErr, sendError))
			if dlqErr != nil {
				logger.L().Error("Critical: Failed to move message to DLQ after retry failure",
					zap.String("notificationID", msgID),
					zap.String("channelType", channelType),
					zap.String("traceID", traceID),
					zap.Error(dlqErr),
				)
			}
		} else {
			metrics.MessagesRetried.WithLabelValues(channelType).Inc() // Use original metric func
		}
	} else {
		// Max retries reached, move to DLQ
		logger.L().Warn("Max retries reached, moving message to DLQ",
			zap.String("notificationID", msgID),
			zap.String("channelType", channelType),
			zap.Int("maxRetries", u.maxRetries),
			zap.String("traceID", traceID),
			zap.Error(sendError), // Include the final error that caused DLQ
		)
		metrics.MessagesDLQ.WithLabelValues(channelType).Inc() // Use original metric func
		dlqErr := msg.MoveToDLQ(ctx, fmt.Errorf("max retries (%d) reached; final error: %w", u.maxRetries, sendError))
		if dlqErr != nil {
			logger.L().Error("Failed to move message to DLQ after max retries",
				zap.String("notificationID", msgID),
				zap.String("channelType", channelType),
				zap.String("traceID", traceID),
				zap.Error(dlqErr),
			)
		}
	}
}
