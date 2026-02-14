package plugins

import (
	"fmt"
)

// GeneratorPlugin defines a small contract for CLI generator plugins. These
// plugins can be registered with the framework (via RegisterGenerator) and
// discovered by the CLI generator wiring. Generator plugins are optional —
// the framework ships built-in generators in internal/generator but users
// can provide custom generators that implement this interface.
type GeneratorPlugin interface {
	Plugin
	// Generate runs the generator into projectRoot passing CLI-style args and
	// returns the list of created files or an error.
	Generate(projectRoot string, args []string) ([]string, error)
	// Help returns a short help string describing the plugin usage.
	Help() string
}

// RegisterGenerator registers a generator plugin. It delegates to the main
// registry.Register and therefore performs the same semantic version
// validation and name collision checks.
func RegisterGenerator(g GeneratorPlugin) error {
	if g == nil {
		return fmt.Errorf("generator: nil plugin")
	}
	return Register(g)
}

// GetGenerator returns a registered GeneratorPlugin by name or nil if not
// found or not a GeneratorPlugin.
func GetGenerator(name string) GeneratorPlugin {
	p := Get(name)
	if p == nil {
		return nil
	}
	if g, ok := p.(GeneratorPlugin); ok {
		return g
	}
	return nil
}

// ListGenerators returns the names of registered plugins that implement the
// GeneratorPlugin contract (in registration order).
func ListGenerators() []string {
	out := List()
	var filtered []string
	for _, name := range out {
		if GetGenerator(name) != nil {
			filtered = append(filtered, name)
		}
	}
	return filtered
}
