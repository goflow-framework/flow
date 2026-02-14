package flow

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Plugin is the canonical plugin interface for Flow. It provides lifecycle
// hooks for initialization, mount-time registration, optional middleware,
// and optional background start/stop semantics.
//
// Versioning contract
// Plugins must expose a semantic version via Version() (for example
// "v0.1.2" or "0.1.2"). The framework validates the plugin version at
// registration time: the plugin's MAJOR version must match the framework's
// PluginAPIMajor for compatibility. The validation is intentionally
// conservative — it rejects malformed versions and major-version
// incompatibilities while allowing any minor/patch level on a matching
// major version.
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

// Sentinel errors related to plugin version validation. These make it easy
// for callers to detect specific failure modes (empty version, invalid
// format, or incompatible major version).
var (
	ErrPluginVersionEmpty      = errors.New("plugin: empty version")
	ErrPluginVersionInvalid    = errors.New("plugin: invalid version")
	ErrPluginIncompatibleMajor = errors.New("plugin: incompatible major version")
)

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

// PluginAPIMajor is the major version of the plugin API expected by this
// version of the framework. Plugins with a differing major semantic version
// are considered incompatible. Bump this constant when making breaking
// changes to the plugin API.
const PluginAPIMajor = 0

// ValidatePluginVersion checks whether a plugin version string (eg "v0.1.2"
// or "0.1.2") is compatible with the framework PluginAPIMajor. It returns
// a sentinel error when the version is empty, invalid, or has an incompatible
// major version so callers can handle specific cases.
func ValidatePluginVersion(v string) error {
	if v == "" {
		return ErrPluginVersionEmpty
	}
	// strip optional leading 'v'
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 1 {
		return fmt.Errorf("%w: %q", ErrPluginVersionInvalid, v)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPluginVersionInvalid, err)
	}

	// optional minor/patch validation: ensure numeric when present
	if len(parts) > 1 {
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return fmt.Errorf("%w: invalid minor: %v", ErrPluginVersionInvalid, err)
		}
	}
	if len(parts) > 2 {
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return fmt.Errorf("%w: invalid patch: %v", ErrPluginVersionInvalid, err)
		}
	}

	if maj != PluginAPIMajor {
		return fmt.Errorf("%w: got %d expected %d", ErrPluginIncompatibleMajor, maj, PluginAPIMajor)
	}
	return nil
}

// ValidatePluginVersionRange validates a plugin version string and ensures
// the MAJOR component falls within [minMajor, maxMajor]. This helper is
// intended for advanced compatibility policies (for example during a
// transition where multiple major versions are temporarily accepted).
func ValidatePluginVersionRange(v string, minMajor, maxMajor int) error {
	if v == "" {
		return ErrPluginVersionEmpty
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 1 {
		return fmt.Errorf("%w: %q", ErrPluginVersionInvalid, v)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPluginVersionInvalid, err)
	}
	if len(parts) > 1 {
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return fmt.Errorf("%w: invalid minor: %v", ErrPluginVersionInvalid, err)
		}
	}
	if len(parts) > 2 {
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return fmt.Errorf("%w: invalid patch: %v", ErrPluginVersionInvalid, err)
		}
	}
	if maj < minMajor || maj > maxMajor {
		return fmt.Errorf("%w: got %d allowed [%d,%d]", ErrPluginIncompatibleMajor, maj, minMajor, maxMajor)
	}
	return nil
}
