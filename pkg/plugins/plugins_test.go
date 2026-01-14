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
func (d *dummyPlugin) Middlewares() []flow.Middleware  { return nil }
func (d *dummyPlugin) Version() string                 { return "v0" }
func (d *dummyPlugin) Start(ctx context.Context) error { return nil }
func (d *dummyPlugin) Stop(ctx context.Context) error  { return nil }

func TestRegisterAndApplyAll(t *testing.T) {
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
