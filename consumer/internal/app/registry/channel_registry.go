package registry

import (
	"fmt"
	"sync"

	"github.com/medeiros-dev/notification-service/consumers/configs"
	"github.com/medeiros-dev/notification-service/consumers/internal/domain/port/channel"
)

// ChannelFactory defines the signature for functions that create channel.Channel instances.
type ChannelFactory func(cfg *configs.Config) (channel.Channel, error)

var (
	channelRegistry = make(map[string]ChannelFactory)
	registryMutex   sync.RWMutex
)

// RegisterChannelFactory registers a new channel factory.
// It should be called during initialization (e.g., in an init() block).
func RegisterChannelFactory(name string, factory ChannelFactory) error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if _, exists := channelRegistry[name]; exists {
		return fmt.Errorf("channel factory already registered: %s", name)
	}
	channelRegistry[name] = factory
	return nil
}

// GetChannelFactory retrieves a channel factory by name.
func GetChannelFactory(name string) (ChannelFactory, error) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	factory, exists := channelRegistry[name]
	if !exists {
		return nil, fmt.Errorf("no channel factory registered for name: %s", name)
	}
	return factory, nil
}
