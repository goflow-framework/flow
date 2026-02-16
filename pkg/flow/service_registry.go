package flow

import (
	"fmt"
	"sync"
)

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

// GetAs returns the service registered under name cast to the requested type T.
// If the service is not found or cannot be asserted to T the zero value and
// false are returned. Implemented as a package-level generic helper because
// Go does not allow methods with independent type parameters on non-generic
// receiver types.
func GetAs[T any](r *ServiceRegistry, name string) (T, bool) {
	var zero T
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[name]
	if !ok {
		return zero, false
	}
	t, ok := s.(T)
	if !ok {
		return zero, false
	}
	return t, true
}

// ListServices returns the registered service names in no particular order.
// It can be used for diagnostics and tests.
func (r *ServiceRegistry) ListServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.services))
	for k := range r.services {
		out = append(out, k)
	}
	return out
}

// Unregister removes a named service from the registry. It returns true if
// a service was removed, or false if no service existed with that name.
func (r *ServiceRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.services[name]; !ok {
		return false
	}
	delete(r.services, name)
	return true
}

// Replace replaces the service registered under name. It returns an error if
// the name is empty or if no service was previously registered under name.
func (r *ServiceRegistry) Replace(name string, svc interface{}) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.services[name]; !ok {
		return fmt.Errorf("service not registered: %s", name)
	}
	r.services[name] = svc
	return nil
}
