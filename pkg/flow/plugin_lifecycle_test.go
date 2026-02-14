package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recPlugin records Init/Mount calls via a callback.
type recPlugin struct {
	name string
	rec  func(string)
}

func (p *recPlugin) Name() string                    { return p.name }
func (p *recPlugin) Version() string                 { return "0.0.1" }
func (p *recPlugin) Init(a *App) error               { p.rec("init:" + p.name); return nil }
func (p *recPlugin) Mount(a *App) error              { p.rec("mount:" + p.name); return nil }
func (p *recPlugin) Middlewares() []Middleware       { return nil }
func (p *recPlugin) Start(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (p *recPlugin) Stop(ctx context.Context) error  { return nil }

// mwPlugin returns a middleware that sets a header named X-PLUGIN to its name.
type mwPlugin struct{ name string }

func (p *mwPlugin) Name() string       { return p.name }
func (p *mwPlugin) Version() string    { return "0.0.1" }
func (p *mwPlugin) Init(a *App) error  { return nil }
func (p *mwPlugin) Mount(a *App) error { return nil }
func (p *mwPlugin) Middlewares() []Middleware {
	return []Middleware{func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-PLUGIN", p.name)
			next.ServeHTTP(w, r)
		})
	}}
}
func (p *mwPlugin) Start(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (p *mwPlugin) Stop(ctx context.Context) error  { return nil }

// startPlugin signals Start and Stop via atomics.
type startPlugin struct{}

func (p *startPlugin) Name() string              { return "starter" }
func (p *startPlugin) Version() string           { return "0.0.1" }
func (p *startPlugin) Init(a *App) error         { return nil }
func (p *startPlugin) Mount(a *App) error        { return nil }
func (p *startPlugin) Middlewares() []Middleware { return nil }
func (p *startPlugin) Start(ctx context.Context) error {
	atomic.StoreInt32(&startedFlag, 1)
	<-ctx.Done()
	return ctx.Err()
}
func (p *startPlugin) Stop(ctx context.Context) error {
	atomic.StoreInt32(&stoppedFlag, 1)
	return nil
}

var (
	startedFlag int32
	stoppedFlag int32
)

// TestInitMountOrder ensures Init and Mount are called in registration order
// (Init for plugin1, Mount for plugin1, then plugin2, etc.).
func TestInitMountOrder(t *testing.T) {
	var mu sync.Mutex
	calls := make([]string, 0)
	rec := func(s string) { mu.Lock(); defer mu.Unlock(); calls = append(calls, s) }

	app := New("test-init-mount")

	if err := app.RegisterPlugin(&recPlugin{name: "one", rec: rec}); err != nil {
		t.Fatalf("register one: %v", err)
	}
	if err := app.RegisterPlugin(&recPlugin{name: "two", rec: rec}); err != nil {
		t.Fatalf("register two: %v", err)
	}

	// expected sequence
	exp := []string{"init:one", "mount:one", "init:two", "mount:two"}

	mu.Lock()
	got := append([]string(nil), calls...)
	mu.Unlock()

	if len(got) != len(exp) {
		t.Fatalf("unexpected call count: got=%v expected=%v", got, exp)
	}
	for i := range exp {
		if got[i] != exp[i] {
			t.Fatalf("mismatch at %d: got=%s expected=%s", i, got[i], exp[i])
		}
	}
}

// TestMiddlewareScoping ensures middleware returned by a plugin is applied
// only to the App that registered it.
func TestMiddlewareScoping(t *testing.T) {
	app1 := New("app1")
	app1.Addr = ":0"
	app2 := New("app2")
	app2.Addr = ":0"

	if err := app1.RegisterPlugin(&mwPlugin{name: "p1"}); err != nil {
		t.Fatalf("register p1: %v", err)
	}

	// app1 should have middleware; app2 should not.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	app1.SetRouter(h)
	app2.SetRouter(h)

	r1 := httptest.NewRequest("GET", "http://example.test/", nil)
	w1 := httptest.NewRecorder()
	app1.Handler().ServeHTTP(w1, r1)
	if got := w1.Header().Get("X-PLUGIN"); got != "p1" {
		t.Fatalf("expected middleware header on app1, got=%q", got)
	}

	r2 := httptest.NewRequest("GET", "http://example.test/", nil)
	w2 := httptest.NewRecorder()
	app2.Handler().ServeHTTP(w2, r2)
	if got := w2.Header().Get("X-PLUGIN"); got != "" {
		t.Fatalf("expected no middleware header on app2, got=%q", got)
	}
}

// TestPluginStartStop ensures Start is invoked automatically when App.Start
// is called, and that Stop is called during Shutdown.
func TestPluginStartStop(t *testing.T) {
	atomic.StoreInt32(&startedFlag, 0)
	atomic.StoreInt32(&stoppedFlag, 0)

	app := New("starter")
	app.Addr = ":0"
	if err := app.RegisterPlugin(&startPlugin{}); err != nil {
		t.Fatalf("register starter: %v", err)
	}

	if err := app.Start(); err != nil {
		t.Fatalf("app start: %v", err)
	}

	// wait for Start to run
	for i := 0; i < 200; i++ {
		if atomic.LoadInt32(&startedFlag) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&startedFlag) != 1 {
		t.Fatalf("plugin Start not invoked")
	}

	// shutdown should call Stop
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if atomic.LoadInt32(&stoppedFlag) != 1 {
		t.Fatalf("plugin Stop not invoked during shutdown")
	}
}
