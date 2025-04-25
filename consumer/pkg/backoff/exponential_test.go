package backoff

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateRetryDelay(t *testing.T) {

	// Using a hypothetical base delay as we cannot mock configs.GetConfig easily.
	const hypotheticalBaseDelayMs = 100
	baseRetryDelay := time.Duration(hypotheticalBaseDelayMs) * time.Millisecond

	tests := []struct {
		name             string
		attempt          int
		expectedMinDelay time.Duration
		expectedMaxDelay time.Duration
		expectZero       bool // For cases where delay should strictly be zero
	}{
		{
			name:       "Attempt 0",
			attempt:    0,
			expectZero: true,
		},
		{
			name:       "Attempt 1",
			attempt:    1,
			expectZero: true,
		},
		{
			name:             "Attempt 2",
			attempt:          2,
			expectedMinDelay: time.Duration(math.Pow(2, float64(2-1)) * float64(baseRetryDelay) * 0.5),
			expectedMaxDelay: time.Duration(math.Pow(2, float64(2-1)) * float64(baseRetryDelay) * 1.5),
		},
		{
			name:             "Attempt 3",
			attempt:          3,
			expectedMinDelay: time.Duration(math.Pow(2, float64(3-1)) * float64(baseRetryDelay) * 0.5),
			expectedMaxDelay: time.Duration(math.Pow(2, float64(3-1)) * float64(baseRetryDelay) * 1.5),
		},
		{
			name:             "Attempt 5",
			attempt:          5,
			expectedMinDelay: time.Duration(math.Pow(2, float64(5-1)) * float64(baseRetryDelay) * 0.5),
			expectedMaxDelay: time.Duration(math.Pow(2, float64(5-1)) * float64(baseRetryDelay) * 1.5),
		},
		{
			name:       "Negative Attempt", // Should be treated as invalid
			attempt:    -1,
			expectZero: true,
		},
		// Add test case for when GetConfig() returns nil (if mockable)
		// Add test case for when baseRetryDelay in config is <= 0 (if mockable)
	}

	// Note: Cannot directly test the case where configs.GetConfig() returns nil
	// or where cfg.BackoffBaseDelay <= 0 without refactoring or complex mocking.
	// We assume GetConfig works and provides a positive BackoffBaseDelay.

	t.Logf("Assuming BaseRetryDelay from config: %v", baseRetryDelay)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// restore := setupTestConfig(hypotheticalBaseDelayMs) // Cannot use this effectively yet
			// defer restore()

			delay := CalculateRetryDelay(tt.attempt, baseRetryDelay)

			if tt.expectZero {
				assert.Equal(t, time.Duration(0), delay, "Expected zero delay")
			} else {
				assert.True(t, delay >= tt.expectedMinDelay, "Delay %v should be >= %v", delay, tt.expectedMinDelay)
				assert.True(t, delay <= tt.expectedMaxDelay, "Delay %v should be <= %v", delay, tt.expectedMaxDelay)
				assert.True(t, delay >= 0, "Delay should never be negative") // Final check
			}
		})
	}
}
