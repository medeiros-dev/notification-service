package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler(t *testing.T) {
	h := MetricsHandler()
	assert.NotNil(t, h)
	assert.Implements(t, (*http.Handler)(nil), h)
}

func TestObserveDuration(t *testing.T) {
	metricName := "notification_consumer_message_processing_duration_seconds"
	metricHelp := "Histogram of message processing duration in seconds, by channel and success status."
	metricLabels := []string{"channel", "success"}

	tests := []struct {
		name        string
		channel     string
		success     bool
		duration    time.Duration
		expectedOut string
	}{
		{
			name:     "Success Email Short Duration",
			channel:  "email",
			success:  true,
			duration: 100 * time.Millisecond,
			expectedOut: fmt.Sprintf(`
				# HELP %s %s
				# TYPE %s histogram
				%s_bucket{channel="email",success="true",le="0.005"} 0
				%s_bucket{channel="email",success="true",le="0.01"} 0
				%s_bucket{channel="email",success="true",le="0.025"} 0
				%s_bucket{channel="email",success="true",le="0.05"} 0
				%s_bucket{channel="email",success="true",le="0.1"} 1
				%s_bucket{channel="email",success="true",le="0.25"} 1
				%s_bucket{channel="email",success="true",le="0.5"} 1
				%s_bucket{channel="email",success="true",le="1"} 1
				%s_bucket{channel="email",success="true",le="2.5"} 1
				%s_bucket{channel="email",success="true",le="5"} 1
				%s_bucket{channel="email",success="true",le="10"} 1
				%s_bucket{channel="email",success="true",le="+Inf"} 1
				%s_sum{channel="email",success="true"} 0.1
				%s_count{channel="email",success="true"} 1
				`, metricName, metricHelp, metricName,
				metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName),
		},
		{
			name:     "Failure SMS Long Duration",
			channel:  "sms",
			success:  false,
			duration: 3 * time.Second,
			expectedOut: fmt.Sprintf(`
				# HELP %s %s
				# TYPE %s histogram
				%s_bucket{channel="sms",success="false",le="0.005"} 0
				%s_bucket{channel="sms",success="false",le="0.01"} 0
				%s_bucket{channel="sms",success="false",le="0.025"} 0
				%s_bucket{channel="sms",success="false",le="0.05"} 0
				%s_bucket{channel="sms",success="false",le="0.1"} 0
				%s_bucket{channel="sms",success="false",le="0.25"} 0
				%s_bucket{channel="sms",success="false",le="0.5"} 0
				%s_bucket{channel="sms",success="false",le="1"} 0
				%s_bucket{channel="sms",success="false",le="2.5"} 0
				%s_bucket{channel="sms",success="false",le="5"} 1
				%s_bucket{channel="sms",success="false",le="10"} 1
				%s_bucket{channel="sms",success="false",le="+Inf"} 1
				%s_sum{channel="sms",success="false"} 3
				%s_count{channel="sms",success="false"} 1
				`, metricName, metricHelp, metricName,
				metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName, metricName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			histVec := prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    metricName,
					Help:    metricHelp,
					Buckets: prometheus.DefBuckets,
				},
				metricLabels,
			)
			reg.MustRegister(histVec)

			successStr := "false"
			if tt.success {
				successStr = "true"
			}

			histVec.WithLabelValues(tt.channel, successStr).Observe(tt.duration.Seconds())

			err := testutil.CollectAndCompare(reg, strings.NewReader(tt.expectedOut), metricName)
			assert.NoError(t, err)
		})
	}
}
