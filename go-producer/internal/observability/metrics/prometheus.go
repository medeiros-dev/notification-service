package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_http_requests_total",
			Help: "Total number of HTTP requests processed, labeled by endpoint and status code.",
		},
		[]string{"endpoint", "status"},
	)

	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "go_producer_http_request_duration_seconds",
			Help:    "Histogram of latencies for HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	KafkaPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_kafka_publish_total",
			Help: "Total number of Kafka publish attempts, labeled by result.",
		},
		[]string{"result"},
	)

	KafkaPublishDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "go_producer_kafka_publish_duration_seconds",
			Help:    "Histogram of Kafka publish durations.",
			Buckets: prometheus.DefBuckets,
		},
	)

	ErrorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_error_total",
			Help: "Total number of errors, labeled by type.",
		},
		[]string{"type"},
	)

	MessagesReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_messages_received_total",
			Help: "Total number of messages received for processing, labeled by channel_type.",
		},
		[]string{"channel_type"},
	)

	MessagesSentSuccessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_messages_sent_success_total",
			Help: "Total number of messages sent successfully to Kafka, labeled by channel_type.",
		},
		[]string{"channel_type"},
	)

	MessagesSentFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_producer_messages_sent_failed_total",
			Help: "Total number of messages failed to send to Kafka, labeled by channel_type.",
		},
		[]string{"channel_type"},
	)
)

func InitMetrics() {
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
	prometheus.MustRegister(KafkaPublishTotal)
	prometheus.MustRegister(KafkaPublishDuration)
	prometheus.MustRegister(ErrorTotal)
	prometheus.MustRegister(MessagesReceivedTotal)
	prometheus.MustRegister(MessagesSentSuccessTotal)
	prometheus.MustRegister(MessagesSentFailedTotal)
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
