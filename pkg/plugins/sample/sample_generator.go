package sample

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goflow-framework/flow/pkg/flow"
	"github.com/goflow-framework/flow/pkg/plugins"
)

// sampleGenerator is a tiny example GeneratorPlugin demonstrating how a
// third-party generator can be implemented and registered. It writes a
// single file named SAMPLE_GENERATED.txt into the target project root.
type sampleGenerator struct{}

func (s *sampleGenerator) Name() string                    { return "samplegen" }
func (s *sampleGenerator) Version() string                 { return "0.0.1" }
func (s *sampleGenerator) Init(app *flow.App) error        { return nil }
func (s *sampleGenerator) Mount(app *flow.App) error       { return nil }
func (s *sampleGenerator) Middlewares() []flow.Middleware  { return nil }
func (s *sampleGenerator) Start(ctx context.Context) error { return nil }
func (s *sampleGenerator) Stop(ctx context.Context) error  { return nil }

func (s *sampleGenerator) Generate(projectRoot string, args []string) ([]string, error) {
	outPath := filepath.Join(projectRoot, "SAMPLE_GENERATED.txt")
	if err := os.WriteFile(outPath, []byte("sample generator output\n"), 0o644); err != nil {
		return nil, fmt.Errorf("samplegen: write failed: %w", err)
	}
	return []string{outPath}, nil
}

func (s *sampleGenerator) Help() string {
	return "samplegen: generates a SAMPLE_GENERATED.txt into the target project root"
}

// Register registers the sample generator with the provided App at runtime.
// This complements the package-level init() registration used for compile-time
// discovery.
func Register(app *flow.App) error {
	if app == nil {
		return fmt.Errorf("app: nil")
	}
	return app.RegisterPlugin(&sampleGenerator{})
}

// Register the sample generator when this package is imported. Real plugins
// should be provided by third-party packages and register in their own init().
func init() {
	// Registering via plugins.RegisterGenerator performs the same semantic
	// version validation and name collision checks as other plugins.
	_ = plugins.RegisterGenerator(&sampleGenerator{})
}
