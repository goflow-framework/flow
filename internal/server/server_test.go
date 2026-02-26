package server

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// TestStartShutdown ensures the Server Start and Shutdown lifecycle methods
// are callable and transition the internal started flag as expected.
func TestStartShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	s := New(handler, "127.0.0.1:0", 1*time.Second, 1*time.Second, 1*time.Second)

	if atomic.LoadInt32(&s.started) != 0 {
		t.Fatalf("expected started==0 before Start, got %d", s.started)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error from Start: %v", err)
	}

	if atomic.LoadInt32(&s.started) != 1 {
		t.Fatalf("expected started==1 after Start, got %d", s.started)
	}

	// Shutdown should succeed quickly
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	// After Shutdown ListenAndServe goroutine should exit and started reset to 0
	// give a small grace period for goroutine to update flag
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&s.started) != 0 {
		t.Fatalf("expected started==0 after Shutdown, got %d", s.started)
	}
}

// TestStartTwice verifies starting an already-started Server returns an error.
func TestStartTwice(t *testing.T) {
	s := New(http.NotFoundHandler(), "127.0.0.1:0", 1*time.Second, 1*time.Second, 1*time.Second)
	if err := s.Start(); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	// second start should return an error
	if err := s.Start(); err == nil {
		// attempt cleanup
		_ = s.Close()
		t.Fatalf("expected error on second Start, got nil")
	}
	// shutdown to clean up
	_ = s.Close()
}

// TestRunCancels ensures Run respects context cancellation and calls the
// provided onShutdown callback.
func TestRunCancels(t *testing.T) {
	s := New(http.NotFoundHandler(), "127.0.0.1:0", 1*time.Second, 1*time.Second, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	var called int32
	doneCh := make(chan error, 1)
	go func() {
		// onShutdown should be invoked after Shutdown completes
		err := s.Run(ctx, 2*time.Second, func(_ context.Context) error {
			atomic.StoreInt32(&called, 1)
			return nil
		})
		doneCh <- err
	}()

	// give server time to start
	time.Sleep(100 * time.Millisecond)

	// cancel context to trigger shutdown
	cancel()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run did not return after context cancellation")
	}

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected onShutdown callback to be called")
	}
}
