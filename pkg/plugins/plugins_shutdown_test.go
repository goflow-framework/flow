package plugins

import (
	"context"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
)

// shutdownPlugin is a plugin that records whether its OnShutdown hook was called.
type shutdownPlugin struct {
	called bool
}

func (p *shutdownPlugin) Name() string                    { return "shutdown-test" }
func (p *shutdownPlugin) Version() string                 { return "v0.0.0" }
func (p *shutdownPlugin) Init(a *flow.App) error          { return nil }
func (p *shutdownPlugin) Mount(a *flow.App) error         { return nil }
func (p *shutdownPlugin) Middlewares() []flow.Middleware  { return nil }
func (p *shutdownPlugin) Start(ctx context.Context) error { return nil }
func (p *shutdownPlugin) Stop(ctx context.Context) error {
	// record that Stop was called during ShutdownAll
	p.called = true
	return nil
}

func TestShutdownAll_CallsOnShutdown(t *testing.T) {
	Reset()

	sp := &shutdownPlugin{}
	if err := Register(sp); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	// applying plugins is not required for shutdown hooks, but keep it realistic
	app := flow.New("shutdown-test")
	if err := ApplyAll(app); err != nil {
		t.Fatalf("apply plugins: %v", err)
	}

	if err := ShutdownAll(context.Background()); err != nil {
		t.Fatalf("shutdown all returned error: %v", err)
	}

	if !sp.called {
		t.Fatalf("expected OnShutdown to be called on plugin")
	}
}
