package metrics

import (
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// --- Handler Tests ---

func TestMetricsHandler(t *testing.T) {
	h := MetricsHandler()
	assert.NotNil(t, h)
	assert.Implements(t, (*http.Handler)(nil), h)
}

// --- Prometheus Metrics Table-Driven Tests ---

func TestPrometheusMetrics(t *testing.T) {
	type metricTestCase struct {
		name        string
		collector   prometheus.Collector
		action      func()
		expectedOut string
		metricNames []string
	}

	testCases := []metricTestCase{
		// CounterVec: HTTP Requests
		{
			name: "HttpRequestsTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_http_requests_total",
					Help: "Total number of HTTP requests processed, labeled by endpoint and status code.",
				},
				[]string{"endpoint", "status"},
			),
			action: func() {
				HttpRequestsTotal.WithLabelValues("/api", "200").Add(2)
			},
			expectedOut: `# HELP go_producer_http_requests_total Total number of HTTP requests processed, labeled by endpoint and status code.
# TYPE go_producer_http_requests_total counter
go_producer_http_requests_total{endpoint="/api",status="200"} 2
`,
			metricNames: []string{"go_producer_http_requests_total"},
		},

		// HistogramVec: HTTP Request Duration
		{
			name: "HttpRequestDuration observes correctly",
			collector: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "go_producer_http_request_duration_seconds",
					Help:    "Histogram of latencies for HTTP requests.",
					Buckets: prometheus.DefBuckets,
				},
				[]string{"endpoint"},
			),
			action: func() {
				HttpRequestDuration.WithLabelValues("/api").Observe(0.15)
			},
			expectedOut: `# HELP go_producer_http_request_duration_seconds Histogram of latencies for HTTP requests.
# TYPE go_producer_http_request_duration_seconds histogram
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.005"} 0
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.01"} 0
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.025"} 0
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.05"} 0
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.1"} 0
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.25"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="0.5"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="1"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="2.5"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="5"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="10"} 1
go_producer_http_request_duration_seconds_bucket{endpoint="/api",le="+Inf"} 1
go_producer_http_request_duration_seconds_sum{endpoint="/api"} 0.15
go_producer_http_request_duration_seconds_count{endpoint="/api"} 1
`,
			metricNames: []string{"go_producer_http_request_duration_seconds"},
		},

		// CounterVec: Kafka Publish
		{
			name: "KafkaPublishTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_kafka_publish_total",
					Help: "Total number of Kafka publish attempts, labeled by result.",
				},
				[]string{"result"},
			),
			action: func() {
				KafkaPublishTotal.WithLabelValues("success").Inc()
			},
			expectedOut: `# HELP go_producer_kafka_publish_total Total number of Kafka publish attempts, labeled by result.
# TYPE go_producer_kafka_publish_total counter
go_producer_kafka_publish_total{result="success"} 1
`,
			metricNames: []string{"go_producer_kafka_publish_total"},
		},

		// Histogram: Kafka Publish Duration
		{
			name: "KafkaPublishDuration observes correctly",
			collector: prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "go_producer_kafka_publish_duration_seconds",
					Help:    "Histogram of Kafka publish durations.",
					Buckets: prometheus.DefBuckets,
				},
			),
			action: func() {
				KafkaPublishDuration.Observe(0.2)
			},
			expectedOut: `# HELP go_producer_kafka_publish_duration_seconds Histogram of Kafka publish durations.
# TYPE go_producer_kafka_publish_duration_seconds histogram
go_producer_kafka_publish_duration_seconds_bucket{le="0.005"} 0
go_producer_kafka_publish_duration_seconds_bucket{le="0.01"} 0
go_producer_kafka_publish_duration_seconds_bucket{le="0.025"} 0
go_producer_kafka_publish_duration_seconds_bucket{le="0.05"} 0
go_producer_kafka_publish_duration_seconds_bucket{le="0.1"} 0
go_producer_kafka_publish_duration_seconds_bucket{le="0.25"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="0.5"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="1"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="2.5"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="5"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="10"} 1
go_producer_kafka_publish_duration_seconds_bucket{le="+Inf"} 1
go_producer_kafka_publish_duration_seconds_sum 0.2
go_producer_kafka_publish_duration_seconds_count 1
`,
			metricNames: []string{"go_producer_kafka_publish_duration_seconds"},
		},

		// CounterVec: Error
		{
			name: "ErrorTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_error_total",
					Help: "Total number of errors, labeled by type.",
				},
				[]string{"type"},
			),
			action: func() {
				ErrorTotal.WithLabelValues("timeout").Inc()
			},
			expectedOut: `# HELP go_producer_error_total Total number of errors, labeled by type.
# TYPE go_producer_error_total counter
go_producer_error_total{type="timeout"} 1
`,
			metricNames: []string{"go_producer_error_total"},
		},

		// CounterVec: Messages Received
		{
			name: "MessagesReceivedTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_messages_received_total",
					Help: "Total number of messages received for processing, labeled by channel_type.",
				},
				[]string{"channel_type"},
			),
			action: func() {
				MessagesReceivedTotal.WithLabelValues("email").Inc()
			},
			expectedOut: `# HELP go_producer_messages_received_total Total number of messages received for processing, labeled by channel_type.
# TYPE go_producer_messages_received_total counter
go_producer_messages_received_total{channel_type="email"} 1
`,
			metricNames: []string{"go_producer_messages_received_total"},
		},

		// CounterVec: Messages Sent Success
		{
			name: "MessagesSentSuccessTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_messages_sent_success_total",
					Help: "Total number of messages sent successfully to Kafka, labeled by channel_type.",
				},
				[]string{"channel_type"},
			),
			action: func() {
				MessagesSentSuccessTotal.WithLabelValues("sms").Inc()
			},
			expectedOut: `# HELP go_producer_messages_sent_success_total Total number of messages sent successfully to Kafka, labeled by channel_type.
# TYPE go_producer_messages_sent_success_total counter
go_producer_messages_sent_success_total{channel_type="sms"} 1
`,
			metricNames: []string{"go_producer_messages_sent_success_total"},
		},

		// CounterVec: Messages Sent Failed
		{
			name: "MessagesSentFailedTotal increments correctly",
			collector: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "go_producer_messages_sent_failed_total",
					Help: "Total number of messages failed to send to Kafka, labeled by channel_type.",
				},
				[]string{"channel_type"},
			),
			action: func() {
				MessagesSentFailedTotal.WithLabelValues("push").Inc()
			},
			expectedOut: `# HELP go_producer_messages_sent_failed_total Total number of messages failed to send to Kafka, labeled by channel_type.
# TYPE go_producer_messages_sent_failed_total counter
go_producer_messages_sent_failed_total{channel_type="push"} 1
`,
			metricNames: []string{"go_producer_messages_sent_failed_total"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			// Register the test collector
			reg.MustRegister(tc.collector)

			// Swap the global metric variable to the test collector for isolation
			switch c := tc.collector.(type) {
			case *prometheus.CounterVec:
				switch tc.metricNames[0] {
				case "go_producer_http_requests_total":
					HttpRequestsTotal = c
				case "go_producer_kafka_publish_total":
					KafkaPublishTotal = c
				case "go_producer_error_total":
					ErrorTotal = c
				case "go_producer_messages_received_total":
					MessagesReceivedTotal = c
				case "go_producer_messages_sent_success_total":
					MessagesSentSuccessTotal = c
				case "go_producer_messages_sent_failed_total":
					MessagesSentFailedTotal = c
				}
			case *prometheus.HistogramVec:
				HttpRequestDuration = c
			case prometheus.Histogram:
				KafkaPublishDuration = c
			}

			// Perform the metric action
			tc.action()

			// Validate the metric output
			for _, metricName := range tc.metricNames {
				err := testutil.CollectAndCompare(tc.collector, strings.NewReader(tc.expectedOut), metricName)
				assert.NoError(t, err)
			}
		})
	}
}
