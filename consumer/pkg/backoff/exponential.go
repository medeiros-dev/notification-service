package backoff

import (
	"math"
	"math/rand"
	"time"

	"github.com/medeiros-dev/notification-service/consumers/configs"
)

// CalculateRetryDelay calculates the retry delay using exponential backoff with jitter.
func CalculateRetryDelay(attempt int, baseRetryDelay time.Duration) time.Duration {
	if attempt <= 1 {
		return 0 // No delay for the first attempt or invalid input
	}
	if baseRetryDelay <= 0 {
		return 0
	}

	// Calculate base delay: 2^(attempt-1) * baseDelay
	backoff := math.Pow(2, float64(attempt-1))
	baseDelayCalc := time.Duration(backoff) * baseRetryDelay

	// Calculate jitter: +/- 50% of the base delay
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // Fallback to default-like behavior
	jitterRange := float64(baseDelayCalc) * 0.5
	jitter := time.Duration(rng.Float64()*2*jitterRange - jitterRange) // [-50%, +50%]

	finalDelay := baseDelayCalc + jitter
	if finalDelay < 0 {
		finalDelay = 0
	}
	return finalDelay
}

// Wrapper function to maintain original signature using global config and default rand
func CalculateRetryDelayFromConfig(attempt int) time.Duration {
	baseDelay := time.Duration(configs.GetConfig().BackoffBaseDelay) * time.Millisecond
	return CalculateRetryDelay(attempt, baseDelay) // Use nil to let the func create its own rand
}
