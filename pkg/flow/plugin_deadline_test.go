package flow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── test doubles ──────────────────────────────────────────────────────────────

// hangPlugin.Start blocks until its context is canceled, simulating a plugin
// that performs a permanently blocking operation on startup (e.g. dialing a
// broken remote service with no built-in timeout).
type hangPlugin struct {
	plugName string
	started  atomic.Int32
}

func (p *hangPlugin) Name() string              { return p.plugName }
func (p *hangPlugin) Version() string           { return "0.0.1" }
func (p *hangPlugin) Init(a *App) error         { return nil }
func (p *hangPlugin) Mount(a *App) error        { return nil }
func (p *hangPlugin) Middlewares() []Middleware { return nil }
func (p *hangPlugin) Start(ctx context.Context) error {
	p.started.Store(1)
	<-ctx.Done()
	return ctx.Err()
}
func (p *hangPlugin) Stop(ctx context.Context) error { return nil }

// quickPlugin.Start records the call and returns immediately, simulating a
// healthy plugin that completes its background setup within microseconds.
type quickPlugin struct {
	plugName string
	started  atomic.Int32
}

func (p *quickPlugin) Name() string              { return p.plugName }
func (p *quickPlugin) Version() string           { return "0.0.1" }
func (p *quickPlugin) Init(a *App) error         { return nil }
func (p *quickPlugin) Mount(a *App) error        { return nil }
func (p *quickPlugin) Middlewares() []Middleware { return nil }
func (p *quickPlugin) Start(ctx context.Context) error {
	p.started.Store(1)
	return nil
}
func (p *quickPlugin) Stop(ctx context.Context) error { return nil }

// bufLogger is a thread-safe Logger implementation that captures all Printf
// calls into a strings.Builder so tests can assert on log output.
type bufLogger struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *bufLogger) Printf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(&l.buf, format, args...)
	l.buf.WriteByte('\n')
}

func (l *bufLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestWithPluginStartTimeout_Option verifies that WithPluginStartTimeout wires
// the correct value onto App.PluginStartTimeout.
func TestWithPluginStartTimeout_Option(t *testing.T) {
	t.Run("zero by default", func(t *testing.T) {
		a := New("zero-timeout")
		if a.PluginStartTimeout != 0 {
			t.Fatalf("expected zero default, got %v", a.PluginStartTimeout)
		}
	})

	t.Run("sets positive value", func(t *testing.T) {
		a := New("custom-timeout", WithPluginStartTimeout(5*time.Second))
		if a.PluginStartTimeout != 5*time.Second {
			t.Fatalf("expected 5s, got %v", a.PluginStartTimeout)
		}
	})

	t.Run("negative opt-out is stored as-is", func(t *testing.T) {
		a := New("no-deadline", WithPluginStartTimeout(-1))
		if a.PluginStartTimeout != -1 {
			t.Fatalf("expected -1, got %v", a.PluginStartTimeout)
		}
	})
}

// TestPluginStart_DefaultTimeoutApplied verifies the constant value is sane.
func TestPluginStart_DefaultTimeoutApplied(t *testing.T) {
	if DefaultPluginStartTimeout <= 0 {
		t.Fatal("DefaultPluginStartTimeout must be positive")
	}
	if DefaultPluginStartTimeout < 5*time.Second {
		t.Fatalf("DefaultPluginStartTimeout unexpectedly small: %v", DefaultPluginStartTimeout)
	}
}

// TestPluginStart_FastPlugin asserts that a plugin whose Start returns
// immediately works correctly under the timeout machinery.
func TestPluginStart_FastPlugin(t *testing.T) {
	qp := &quickPlugin{plugName: "quick"}

	a := New("fast-test", WithPluginStartTimeout(50*time.Millisecond))
	a.Addr = ":0"

	if err := a.RegisterPlugin(qp); err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}
	if err := a.Start(); err != nil {
		t.Fatalf("App.Start: %v", err)
	}

	// Wait for plugin Start to be entered.
	dl := time.Now().Add(time.Second)
	for qp.started.Load() == 0 && time.Now().Before(dl) {
		time.Sleep(5 * time.Millisecond)
	}
	if qp.started.Load() == 0 {
		t.Fatal("fast plugin Start was never entered")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// TestPluginStart_HungPluginTimedOut asserts that when a plugin's Start method
// blocks indefinitely its context is force-canceled within PluginStartTimeout
// and the pluginWg is decremented — so Shutdown does not block forever waiting
// for a goroutine that will never exit on its own.
func TestPluginStart_HungPluginTimedOut(t *testing.T) {
	hp := &hangPlugin{plugName: "hung"}

	pluginTimeout := 100 * time.Millisecond
	a := New("timeout-test", WithPluginStartTimeout(pluginTimeout))
	a.Addr = ":0"

	if err := a.RegisterPlugin(hp); err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}
	if err := a.Start(); err != nil {
		t.Fatalf("App.Start: %v", err)
	}

	// Wait until the plugin goroutine has been entered.
	dl := time.Now().Add(2 * time.Second)
	for hp.started.Load() == 0 && time.Now().Before(dl) {
		time.Sleep(5 * time.Millisecond)
	}
	if hp.started.Load() == 0 {
		t.Fatal("plugin Start was never entered")
	}

	// After PluginStartTimeout the goroutine should self-exit.
	// Shutdown should complete well within a generous margin.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), pluginTimeout*5+500*time.Millisecond)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned error after hung plugin should have timed out: %v", err)
	}
}

// TestPluginStart_NegativeTimeoutDisablesDeadline verifies that a negative
// PluginStartTimeout opts out of the per-plugin timeout. The goroutine stays
// alive until Shutdown cancels the pluginCtx.
func TestPluginStart_NegativeTimeoutDisablesDeadline(t *testing.T) {
	hp := &hangPlugin{plugName: "hung-no-deadline"}

	// Negative value disables per-plugin deadline.
	a := New("no-deadline-test", WithPluginStartTimeout(-1))
	a.Addr = ":0"

	if err := a.RegisterPlugin(hp); err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}
	if err := a.Start(); err != nil {
		t.Fatalf("App.Start: %v", err)
	}

	// Wait for Start to be entered.
	dl := time.Now().Add(2 * time.Second)
	for hp.started.Load() == 0 && time.Now().Before(dl) {
		time.Sleep(5 * time.Millisecond)
	}
	if hp.started.Load() == 0 {
		t.Fatal("plugin Start was never entered")
	}

	// Shutdown cancels pluginCtx, which unblocks the hung goroutine.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// TestPluginStart_TimeoutWarningLogged verifies that a timeout warning
// containing the plugin name and "timed out" is emitted to the app logger
// when a hung plugin exceeds its per-plugin start deadline.
func TestPluginStart_TimeoutWarningLogged(t *testing.T) {
	hp := &hangPlugin{plugName: "logcheck"}
	pluginTimeout := 80 * time.Millisecond

	bl := &bufLogger{}
	a := New("log-test", WithPluginStartTimeout(pluginTimeout), WithLogger(bl))
	a.Addr = ":0"

	if err := a.RegisterPlugin(hp); err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}
	if err := a.Start(); err != nil {
		t.Fatalf("App.Start: %v", err)
	}

	// Wait for Start to be entered.
	dl := time.Now().Add(2 * time.Second)
	for hp.started.Load() == 0 && time.Now().Before(dl) {
		time.Sleep(5 * time.Millisecond)
	}

	// Allow the per-plugin timeout to fire and the warning to be logged.
	time.Sleep(pluginTimeout + 200*time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.Shutdown(shutdownCtx)

	logged := bl.String()
	if !strings.Contains(logged, "logcheck") || !strings.Contains(logged, "timed out") {
		t.Fatalf("expected timeout warning in log, got: %q", logged)
	}
}
