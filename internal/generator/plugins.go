package generator

import (
	"github.com/undiegomejia/flow/pkg/plugins"
)

// ListRegisteredGenerators returns the names of registered generator plugins.
// This helper allows the CLI wiring to discover third-party generator
// implementations that have registered via pkg/plugins.RegisterGenerator.
func ListRegisteredGenerators() []string {
	return plugins.ListGenerators()
}

// GetRegisteredGenerator returns a registered GeneratorPlugin by name or nil
// if none exists. This provides an internal-friendly accessor for the CLI
// generator code to invoke external generator plugins.
func GetRegisteredGenerator(name string) plugins.GeneratorPlugin {
	return plugins.GetGenerator(name)
}
