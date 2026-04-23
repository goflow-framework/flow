package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	internalapp "github.com/goflow-framework/flow/internal/app"
)

// newLC is a helper that builds a Lifecycle on a random free port.
func newLC(t *testing.T) *internalapp.Lifecycle {
	t.Helper()
	return internalapp.NewLifecycle(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		":0", // OS assigns a free port
		5*time.Second,
		10*time.Second,
		120*time.Second,
		nil, // use default logger
	)
}

func TestLifecycle_StartShutdown(t *testing.T) {
	lc := newLC(t)

	if lc.IsRunning() {
		t.Fatal("IsRunning() = true before Start, want false")
	}

	if err := lc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !lc.IsRunning() {
		t.Error("IsRunning() = false after Start, want true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := lc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if lc.IsRunning() {
		t.Error("IsRunning() = true after Shutdown, want false")
	}
}

func TestLifecycle_StartTwice(t *testing.T) {
	lc := newLC(t)

	if err := lc.Start(); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = lc.Shutdown(ctx)
	})

	err := lc.Start()
	if !errors.Is(err, internalapp.ErrAlreadyRunning) {
		t.Errorf("second Start() error = %v, want ErrAlreadyRunning", err)
	}
}

func TestLifecycle_ShutdownBeforeStart(t *testing.T) {
	lc := newLC(t)
	ctx := context.Background()
	// Shutdown on a not-yet-started lifecycle must be a no-op.
	if err := lc.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() before Start error = %v, want nil", err)
	}
}

func TestLifecycle_ShutdownTwice(t *testing.T) {
	lc := newLC(t)
	if err := lc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := lc.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}
	// second call must be idempotent
	if err := lc.Shutdown(ctx); err != nil {
		t.Errorf("second Shutdown() error = %v, want nil", err)
	}
}

func TestLifecycle_RunCancels(t *testing.T) {
	lc := newLC(t)

	shutdownCalled := false
	hook := func(_ context.Context) error {
		shutdownCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- lc.Run(ctx, 3*time.Second, hook)
	}()

	// Give the server a moment to start.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after context cancel")
	}

	if !shutdownCalled {
		t.Error("onShutdown hook was not called")
	}
}

func TestLifecycle_RunStartError(t *testing.T) {
	// Start a first lifecycle to occupy a port, then try to start a second
	// one on the same port — Run must propagate the Start error.
	first := newLC(t)
	if err := first.Start(); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = first.Shutdown(ctx)
	})

	// Try to bind the same address (will already be running, so ErrAlreadyRunning).
	second := newLC(t)
	if err := second.Start(); err != nil {
		t.Fatalf("second Start() unexpected error = %v", err)
	}
	// Now trying to start it again must return ErrAlreadyRunning.
	err := second.Run(context.Background(), time.Second)
	if !errors.Is(err, internalapp.ErrAlreadyRunning) {
		t.Errorf("Run() on already-running lifecycle = %v, want ErrAlreadyRunning", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = second.Shutdown(ctx)
}

func TestLifecycle_OnShutdownHookError(t *testing.T) {
	lc := newLC(t)
	if err := lc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	hookErr := errors.New("hook failed")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := lc.Shutdown(ctx, func(_ context.Context) error {
		return hookErr
	})
	if !errors.Is(err, hookErr) {
		t.Errorf("Shutdown() = %v, want hookErr", err)
	}
}

func TestLifecycle_AddrDefault(t *testing.T) {
	lc := internalapp.NewLifecycle(http.DefaultServeMux, "", 0, 0, 0, nil)
	if lc.Addr() != ":3000" {
		t.Errorf("Addr() = %q, want :3000", lc.Addr())
	}
}

func TestLifecycle_OnShutdownNilHookSkipped(t *testing.T) {
	lc := newLC(t)
	if err := lc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Passing a nil hook must not panic.
	if err := lc.Shutdown(ctx, nil); err != nil {
		t.Errorf("Shutdown() with nil hook error = %v, want nil", err)
	}
}
