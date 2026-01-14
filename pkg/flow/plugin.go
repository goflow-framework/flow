package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Plugin is the canonical plugin interface for Flow. It provides lifecycle
// hooks for initialization, mount-time registration, optional middleware,
// and optional background start/stop semantics.
type Plugin interface {
	Name() string
	Version() string
	Init(app *App) error
	Mount(app *App) error
	Middlewares() []Middleware
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

// GetAs returns the service registered under name cast to the requested type T.
// If the service is not found or cannot be asserted to T the zero value and
// false are returned. This is a convenience wrapper that avoids repeating
// type assertions at call sites.
// GetAs returns the service registered under name cast to the requested type T.
// It is implemented as a generic package-level helper because Go does not
// allow methods with independent type parameters on non-generic receiver
// types. If the service is not found or cannot be asserted to T the zero
// value and false are returned.
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

// PluginAPIMajor is the major version of the plugin API expected by this
// version of the framework. Plugins with a differing major semantic version
// should be considered incompatible.
const PluginAPIMajor = 0

// ValidatePluginVersion checks whether a plugin version string (eg "v0.1.2"
// or "0.1.2") is compatible with the framework PluginAPIMajor. It returns
// an error if the version cannot be parsed or the major version doesn't
// match PluginAPIMajor.
func ValidatePluginVersion(v string) error {
	if v == "" {
		return fmt.Errorf("empty plugin version")
	}
	// strip optional leading 'v'
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid plugin version: %q", v)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid major version in %q: %w", v, err)
	}
	if maj != PluginAPIMajor {
		return fmt.Errorf("incompatible plugin major version %d (expected %d)", maj, PluginAPIMajor)
	}
	return nil
}
