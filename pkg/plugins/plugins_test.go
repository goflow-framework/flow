package plugins

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/undiegomejia/flow/pkg/flow"
)

type dummyPlugin struct {
	name        string
	initCalled  bool
	mountCalled bool
}

func (d *dummyPlugin) Name() string { return d.name }
func (d *dummyPlugin) Init(a *flow.App) error {
	d.initCalled = true
	// mutate the app to prove Init gets the App reference
	a.Name = "inited-" + d.name
	return nil
}
func (d *dummyPlugin) Mount(a *flow.App) error {
	d.mountCalled = true
	return nil
}
func (d *dummyPlugin) Middlewares() []flow.Middleware { return nil }

func TestRegisterAndApplyAll(t *testing.T) {
	// ensure clean registry for test
	Reset()
	defer Reset()

	app := flow.New("foo")
	name := fmt.Sprintf("dummy-%d", time.Now().UnixNano())
	p := &dummyPlugin{name: name}

	if err := Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	// ensure plugin is visible in List/Get
	ls := List()
	found := false
	for _, n := range ls {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("registered plugin not listed")
	}

	if got := Get(name); got == nil {
		t.Fatalf("Get returned nil for registered plugin")
	}

	if err := ApplyAll(app); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}

	if !p.initCalled || !p.mountCalled {
		t.Fatalf("plugin hooks not called: init=%v mount=%v", p.initCalled, p.mountCalled)
	}

	if app.Name != "inited-"+name {
		t.Fatalf("expected app name mutated by plugin Init, got %q", app.Name)
	}
}

type shutdownPlugin struct {
	dummyPlugin
	shutdownCalled bool
}

// Implement the expected method via type with correct signature for the
// test: provide OnShutdown(context.Context) error so ShutdownAll can detect it.
func (s *shutdownPlugin) OnShutdown(ctx context.Context) error {
	s.shutdownCalled = true
	return nil
}

func TestShutdownAll(t *testing.T) {
	Reset()
	defer Reset()

	app := flow.New("foo")
	name := fmt.Sprintf("shutdown-%d", time.Now().UnixNano())
	sp := &shutdownPlugin{dummyPlugin: dummyPlugin{name: name}}

	if err := Register(sp); err != nil {
		t.Fatalf("register shutdown plugin: %v", err)
	}

	// apply so mount/init are called (not strictly required for shutdown)
	if err := ApplyAll(app); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}

	if err := ShutdownAll(context.Background()); err != nil {
		t.Fatalf("ShutdownAll: %v", err)
	}

	if !sp.shutdownCalled {
		t.Fatalf("expected shutdown hook called")
	}
}
