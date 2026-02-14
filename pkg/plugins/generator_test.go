package plugins

import (
	"context"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
)

// mockGenerator is a simple GeneratorPlugin used for tests.
type mockGenerator struct{}

func (m *mockGenerator) Name() string                    { return "mockgen" }
func (m *mockGenerator) Version() string                 { return "0.0.1" }
func (m *mockGenerator) Init(app *flow.App) error        { return nil }
func (m *mockGenerator) Mount(app *flow.App) error       { return nil }
func (m *mockGenerator) Middlewares() []flow.Middleware  { return nil }
func (m *mockGenerator) Start(ctx context.Context) error { return nil }
func (m *mockGenerator) Stop(ctx context.Context) error  { return nil }
func (m *mockGenerator) Generate(projectRoot string, args []string) ([]string, error) {
	return []string{"a.txt"}, nil
}
func (m *mockGenerator) Help() string { return "mock help" }

func TestRegisterAndGetGenerator(t *testing.T) {
	Reset()
	g := &mockGenerator{}
	if err := RegisterGenerator(g); err != nil {
		t.Fatalf("RegisterGenerator failed: %v", err)
	}

	// list should include mockgen
	found := false
	for _, name := range ListGenerators() {
		if name == g.Name() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("registered generator not listed")
	}

	got := GetGenerator(g.Name())
	if got == nil {
		t.Fatalf("GetGenerator returned nil")
	}

	files, err := got.Generate("/tmp", nil)
	if err != nil || len(files) != 1 || files[0] != "a.txt" {
		t.Fatalf("Generate did not behave as expected: %v %v", files, err)
	}
}
