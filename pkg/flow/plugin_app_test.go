package flow

import (
	"context"
	"testing"
)

type testPlugin struct{ name string }

func (p *testPlugin) Name() string    { return p.name }
func (p *testPlugin) Version() string { return "0.0.1" }
func (p *testPlugin) Init(a *App) error {
	// register a per-app service to prove isolation
	if a == nil {
		return nil
	}
	return a.RegisterService("svc-"+p.name, p)
}
func (p *testPlugin) Mount(a *App) error              { return nil }
func (p *testPlugin) Middlewares() []Middleware       { return nil }
func (p *testPlugin) Start(ctx context.Context) error { return nil }
func (p *testPlugin) Stop(ctx context.Context) error  { return nil }

func TestAppScopedPluginIsolation(t *testing.T) {
	a1 := New("app1")
	a2 := New("app2")

	if err := a1.RegisterPlugin(&testPlugin{name: "tp"}); err != nil {
		t.Fatalf("RegisterPlugin failed for a1: %v", err)
	}
	if err := a2.RegisterPlugin(&testPlugin{name: "tp"}); err != nil {
		t.Fatalf("RegisterPlugin failed for a2: %v", err)
	}

	s1, ok1 := a1.GetService("svc-tp")
	if !ok1 {
		t.Fatalf("expected service svc-tp in a1")
	}
	s2, ok2 := a2.GetService("svc-tp")
	if !ok2 {
		t.Fatalf("expected service svc-tp in a2")
	}
	if s1 == s2 {
		t.Fatalf("expected services to be distinct across apps")
	}

	// registering the same plugin name twice on the same app should fail
	if err := a1.RegisterPlugin(&testPlugin{name: "tp"}); err == nil {
		t.Fatalf("expected error when registering duplicate plugin on same app")
	}
}
