package flow

import (
    "context"
    "testing"
    "time"

    execpkg "github.com/undiegomejia/flow/pkg/exec"
)

// TestBoundedExecutor_BlocksWhenFull verifies Submit blocks when the worker
// count + queue are completely occupied, and that it completes once space
// becomes available.
func TestBoundedExecutor_BlocksWhenFull(t *testing.T) {
    be := NewBoundedExecutor(1, 1)
    // block the single worker until we close 'start'
    start := make(chan struct{})
    if err := be.Submit(context.Background(), func(ctx context.Context) { <-start }); err != nil {
        t.Fatalf("initial submit failed: %v", err)
    }
    // fill the queue
    if err := be.Submit(context.Background(), func(ctx context.Context) {}); err != nil {
        t.Fatalf("second submit failed: %v", err)
    }

    done := make(chan error, 1)
    go func() {
        // this submit should block until we release 'start'
        done <- be.Submit(context.Background(), func(ctx context.Context) {})
    }()

    select {
    case <-done:
        t.Fatalf("expected submit to block when queue is full")
    case <-time.After(30 * time.Millisecond):
        // expected: still blocked
    }

    // release the running task so the queued submission can make progress
    close(start)

    select {
    case err := <-done:
        if err != nil {
            t.Fatalf("submit returned error after queue freed: %v", err)
        }
    case <-time.After(200 * time.Millisecond):
        t.Fatalf("submit did not complete after queue freed")
    }

    _ = be.Shutdown(context.Background())
}

// TestBoundedExecutor_ShutdownRejectsSubmits ensures that after Shutdown is
// initiated the executor rejects new submissions with ErrExecutorClosed.
func TestBoundedExecutor_ShutdownRejectsSubmits(t *testing.T) {
    be := NewBoundedExecutor(1, 1)
    if err := be.Submit(context.Background(), func(ctx context.Context) { time.Sleep(30 * time.Millisecond) }); err != nil {
        t.Fatalf("initial submit failed: %v", err)
    }

    // start shutdown concurrently
    shutdownDone := make(chan error, 1)
    go func() {
        shutdownDone <- be.Shutdown(context.Background())
    }()

    // Give shutdown a moment to set the closed flag
    time.Sleep(5 * time.Millisecond)

    if err := be.Submit(context.Background(), func(ctx context.Context) {}); err != execpkg.ErrExecutorClosed {
        t.Fatalf("expected ErrExecutorClosed after shutdown, got: %v", err)
    }

    if err := <-shutdownDone; err != nil {
        t.Fatalf("shutdown failed: %v", err)
    }
}
