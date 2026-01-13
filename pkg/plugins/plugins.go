package plugins

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/undiegomejia/flow/pkg/flow"
)

// Plugin is the minimal interface a Flow plugin should implement.
// Implementations may provide initialization hooks, mount-time hooks and
// optional middleware to be applied to the host App.
type Plugin interface {
	// Name returns a short unique name for the plugin.
	Name() string

	// Init is called early with the App reference. Use it to configure
	// global resources or perform early checks. It may mutate the App.
	Init(a *flow.App) error

	// Mount is called after Init and after previously-registered plugins
	// have been initialized. Use it to register routes or attach middleware
	// via App.Use().
	Mount(a *flow.App) error

	// Middlewares returns middleware the plugin wishes to register. These
	// will be appended to the App in Mount order. Returning nil is fine.
	Middlewares() []flow.Middleware
}

var (
	mu       sync.RWMutex
	registry = make(map[string]Plugin)
	order    = make([]string, 0)
)

var (
	// ErrPluginExists is returned when attempting to register a plugin with
	// a name that's already registered.
	ErrPluginExists = errors.New("plugin: plugin already registered")
)

// Register registers a plugin for later application. It is safe to call from
// init() functions in plugin packages.
func Register(p Plugin) error {
	if p == nil {
		return errors.New("plugin: nil plugin")
	}
	name := p.Name()
	if name == "" {
		return errors.New("plugin: empty name")
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		return ErrPluginExists
	}
	registry[name] = p
	order = append(order, name)
	return nil
}

// List returns the registered plugin names in registration order.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, len(order))
	copy(out, order)
	return out
}

// Get returns a plugin by name or nil if not found.
func Get(name string) Plugin {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}

// ApplyAll runs Init and Mount hooks for all registered plugins in
// registration order. If any Init or Mount returns an error the process
// stops and the error is returned. Middlewares returned by a plugin are
// registered on the App during Mount (after calling Mount).
func ApplyAll(a *flow.App) error {
	mu.RLock()
	names := make([]string, len(order))
	copy(names, order)
	mu.RUnlock()

	// First pass: Init all plugins in order.
	for _, name := range names {
		p := Get(name)
		if p == nil {
			continue
		}
		if err := p.Init(a); err != nil {
			return err
		}
	}

	// Second pass: Mount all plugins in order and register their middleware.
	for _, name := range names {
		p := Get(name)
		if p == nil {
			continue
		}
		if err := p.Mount(a); err != nil {
			return err
		}
		if mws := p.Middlewares(); mws != nil {
			for _, mw := range mws {
				a.Use(mw)
			}
		}
	}

	return nil
}

// Reset clears the plugin registry. Useful for tests that register plugins
// during execution. It is safe to call concurrently with Register, but
// callers should avoid racing with ApplyAll.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]Plugin)
	order = make([]string, 0)
}

// ShutdownAll invokes optional shutdown hooks on registered plugins in the
// reverse of registration order. A plugin may implement an OnShutdown
// method with the signature `OnShutdown(context.Context) error` — if present
// it will be called. ShutdownAll returns the first error encountered.
func ShutdownAll(ctx context.Context) error {
	mu.RLock()
	names := make([]string, len(order))
	copy(names, order)
	mu.RUnlock()

	// iterate in reverse order for shutdown
	var errs []string
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		p := Get(name)
		if p == nil {
			continue
		}
		// optional shutdown hook
		if sh, ok := p.(interface{ OnShutdown(context.Context) error }); ok {
			if err := sh.OnShutdown(ctx); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errors.New("plugin shutdown error: " + errs[0])
	default:
		return errors.New("plugin shutdown errors: " + strings.Join(errs, "; "))
	}
}
