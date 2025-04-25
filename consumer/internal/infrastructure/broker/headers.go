package broker

import (
	"strconv"

	"github.com/medeiros-dev/notification-service/consumers/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Constants for header keys
const (
	RetryHeader     = "x-retry-count"
	dlqReasonHeader = "x-dlq-reason"
)

// getRetryCount extracts the retry count from Kafka message headers.
func getRetryCount(headers []kafka.Header) int {
	for _, h := range headers {
		if h.Key == RetryHeader {
			count, err := strconv.Atoi(string(h.Value))
			if err == nil {
				return count
			}
			logger.L().Warn("Invalid retry header value",
				zap.String("headerKey", RetryHeader),
				zap.ByteString("headerValue", h.Value),
				zap.Error(err),
			)
			// Return 0 if header is invalid
			return 0
		}
	}
	// Return 0 if header is not present
	return 0
}

// updateRetryHeader adds or updates the retry count header.
func updateRetryHeader(headers []kafka.Header, retryCount int) []kafka.Header {
	newHeaders := make([]kafka.Header, 0, len(headers)+1)
	foundHeader := false
	retryValue := []byte(strconv.Itoa(retryCount))

	for _, h := range headers {
		if h.Key == RetryHeader {
			newHeaders = append(newHeaders, kafka.Header{Key: RetryHeader, Value: retryValue})
			foundHeader = true
		} else {
			// Keep other headers
			newHeaders = append(newHeaders, h)
		}
	}
	if !foundHeader {
		newHeaders = append(newHeaders, kafka.Header{Key: RetryHeader, Value: retryValue})
	}
	return newHeaders
}
