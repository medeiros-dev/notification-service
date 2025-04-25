package registry

import (
	"context"
	"testing"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Channel implementation for testing
type MockChannel struct{}

func (m *MockChannel) Send(ctx context.Context, message string, destination string) error {
	return nil
}

// Mock Factory function
func mockFactory(cfg *configs.Config) (channel.Channel, error) {
	return &MockChannel{}, nil
}

// Helper to reset the registry state before each test
func resetRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	channelRegistry = make(map[string]ChannelFactory)
}

func TestRegisterChannelFactory(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry) // Ensure cleanup after test completes

	t.Run("Register New Factory", func(t *testing.T) {
		err := RegisterChannelFactory("test-channel", mockFactory)
		assert.NoError(t, err)

		// Verify it's in the map (internal check, normally test via Get)
		registryMutex.RLock()
		_, exists := channelRegistry["test-channel"]
		registryMutex.RUnlock()
		assert.True(t, exists)
	})

	t.Run("Register Duplicate Factory", func(t *testing.T) {
		// First registration (ensure it's there)
		_ = RegisterChannelFactory("duplicate-channel", mockFactory)

		// Second registration attempt
		err := RegisterChannelFactory("duplicate-channel", mockFactory)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestGetChannelFactory(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry)

	t.Run("Get Existing Factory", func(t *testing.T) {
		// Register first
		err := RegisterChannelFactory("get-channel", mockFactory)
		require.NoError(t, err)

		factory, err := GetChannelFactory("get-channel")
		assert.NoError(t, err)
		assert.NotNil(t, factory)

		// Optional: Execute the factory to ensure it returns the mock channel
		instance, err := factory(nil) // Pass nil config, mockFactory doesn't use it
		assert.NoError(t, err)
		assert.IsType(t, &MockChannel{}, instance)
	})

	t.Run("Get Non-Existent Factory", func(t *testing.T) {
		_, err := GetChannelFactory("non-existent-channel")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no channel factory registered")
	})
}

// Note: Concurrency tests could be added if high contention on the registry is expected,
// but given the typical use case (registration at init), these basic tests cover functionality.
