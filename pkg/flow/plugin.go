package flow

import (
	"context"
	"fmt"
	"sync"
)

// Plugin gives lifecycle hooks for apps. Plugins may be registered at
// application initialization time and optionally started/stopped with the App.
type Plugin interface {
	Name() string
	Version() string
	Init(app *App) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ServiceRegistry is a simple, threadsafe registry for sharing services
// between application components and plugins. Services are stored as
// interface{} and consumers are expected to assert the concrete type.
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]interface{}
}

// NewServiceRegistry creates an empty ServiceRegistry.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{services: make(map[string]interface{})}
}

// Register registers a service under the provided name. If a service is
// already registered with the same name an error is returned.
func (r *ServiceRegistry) Register(name string, svc interface{}) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.services[name]; ok {
		return fmt.Errorf("service already registered: %s", name)
	}
	r.services[name] = svc
	return nil
}

// Get returns the service and a boolean indicating whether it was found.
func (r *ServiceRegistry) Get(name string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[name]
	return s, ok
}
