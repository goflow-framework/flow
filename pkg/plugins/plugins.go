package plugins

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/goflow-framework/flow/pkg/flow"
)

// Plugin is an alias to the canonical flow.Plugin interface. This package
// provides a small registry and helpers for compile-time plugin registration
// used by the framework and examples.
type Plugin = flow.Plugin

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

	// Validate compatible plugin API version before registering. Use errors.Is
	// to allow callers to detect specific failure kinds (empty, invalid,
	// incompatible) via the framework sentinel errors.
	if err := flow.ValidatePluginVersion(p.Version()); err != nil {
		return fmt.Errorf("plugin %s: %w", name, err)
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

	// If an App is provided prefer to register plugins into the App's
	// scoped registry so plugin state (and middleware) is isolated per App.
	// App.RegisterPlugin will validate the version and run Init/Mount and
	// register middleware on the App. This keeps behavior consistent with
	// runtime registration patterns.
	if a == nil {
		return fmt.Errorf("plugins: nil app provided to ApplyAll")
	}

	for _, name := range names {
		p := Get(name)
		if p == nil {
			continue
		}
		if err := a.RegisterPlugin(p); err != nil {
			return err
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

// ShutdownAll invokes plugin shutdown hooks in reverse registration order.
// It will call the canonical Stop(context.Context) error lifecycle method
// defined on the framework Plugin interface. ShutdownAll returns the first
// aggregated error encountered (as a single error containing messages).
func ShutdownAll(ctx context.Context) error {
	mu.RLock()
	names := make([]string, len(order))
	copy(names, order)
	mu.RUnlock()

	// iterate in reverse order for shutdown and call Stop(ctx) on each
	var errs []string
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		p := Get(name)
		if p == nil {
			continue
		}
		if err := p.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
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
