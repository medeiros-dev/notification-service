package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestL(t *testing.T) {
	// Ensure L returns the current global logger
	InitializeLogger(true) // Ensure log is initialized
	assert.NotNil(t, L())
	assert.Same(t, log, L())
}
