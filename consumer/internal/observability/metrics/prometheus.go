package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Define buckets for message processing duration histogram (e.g., 1ms to 30s)
	durationBuckets = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30}

	// MessagesReceived counts total messages received from the broker (after unmarshalling)
	MessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_consumer_messages_received_total",
			Help: "Total number of messages successfully received and unmarshalled from the broker, by channel.",
		},
		[]string{"channel"}, // Label: channel type (e.g., "email", "sms")
	)

	// MessagesProcessed counts successfully processed messages
	MessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_consumer_messages_processed_total",
			Help: "Total number of messages successfully processed (sent notification), by channel.",
		},
		[]string{"channel"},
	)

	// MessagesFailed counts messages that failed processing after initial attempt (before retries)
	MessagesFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_consumer_messages_failed_total",
			Help: "Total number of messages that failed processing on the initial attempt, by channel.",
		},
		[]string{"channel"},
	)

	// MessagesRetried counts messages sent for retry
	MessagesRetried = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_consumer_messages_retried_total",
			Help: "Total number of messages sent for retry, by channel.",
		},
		[]string{"channel"},
	)

	// MessagesDLQ counts messages moved to the Dead Letter Queue
	MessagesDLQ = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_consumer_messages_dlq_total",
			Help: "Total number of messages moved to the Dead Letter Queue, by channel.",
		},
		[]string{"channel"},
	)

	// ProcessingDuration measures the duration of message processing attempts
	ProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_consumer_message_processing_duration_seconds",
			Help:    "Histogram of message processing duration in seconds, by channel and success status.",
			Buckets: durationBuckets,
		},
		[]string{"channel", "success"}, // Labels: channel type, success ("true" or "false")
	)
)

// MetricsHandler returns the HTTP handler for the Prometheus metrics endpoint.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// ObserveDuration simplifies observing processing duration.
func ObserveDuration(channelType string, success bool, start time.Time) {
	duration := time.Since(start).Seconds()
	successStr := "false"
	if success {
		successStr = "true"
	}
	ProcessingDuration.WithLabelValues(channelType, successStr).Observe(duration)
}
