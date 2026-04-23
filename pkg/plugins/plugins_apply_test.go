package plugins

import (
	"context"
	"testing"

	"github.com/goflow-framework/flow/pkg/flow"
)

// testPlugin is a small plugin used by the tests. Declared at package level so
// methods can be attached.
type testPlugin struct{}

func (p *testPlugin) Name() string    { return "test-sample" }
func (p *testPlugin) Version() string { return "v0.0.0" }
func (p *testPlugin) Init(a *flow.App) error {
	return a.RegisterService("sample.value", "hello-from-sample-plugin")
}
func (p *testPlugin) Mount(a *flow.App) error         { return nil }
func (p *testPlugin) Middlewares() []flow.Middleware  { return nil }
func (p *testPlugin) Start(ctx context.Context) error { return nil }
func (p *testPlugin) Stop(ctx context.Context) error  { return nil }

// TestApplyAll_WithSample verifies that a registered plugin is applied and
// that it registers a service into the app's ServiceRegistry.
func TestApplyAll_WithSample(t *testing.T) {
	// ensure clean registry in case other tests ran
	Reset()

	// register our test plugin and apply
	if err := Register(&testPlugin{}); err != nil {
		t.Fatalf("register test plugin: %v", err)
	}

	app := flow.New("test-apply")
	if err := ApplyAll(app); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	v, ok := app.GetService("sample.value")
	if !ok {
		t.Fatalf("expected sample.value service registered")
	}

	s, _ := v.(string)
	if s != "hello-from-sample-plugin" {
		t.Fatalf("unexpected sample.value: %q", s)
	}
}
