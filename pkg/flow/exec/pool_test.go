package flowexec

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	execpkg "github.com/undiegomejia/flow/pkg/exec"
)

// TestSubmitAndDrain verifies that all submitted tasks run to completion before
// Shutdown returns.
func TestSubmitAndDrain(t *testing.T) {
	const numTasks = 50
	be := NewBoundedExecutor(4, numTasks)

	var count atomic.Int64
	for i := 0; i < numTasks; i++ {
		if err := be.Submit(context.Background(), func(_ context.Context) {
			count.Add(1)
		}); err != nil {
			t.Fatalf("Submit: unexpected error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := be.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}

	if got := count.Load(); got != numTasks {
		t.Errorf("expected %d tasks to run, got %d", numTasks, got)
	}
}

// TestSubmitAfterShutdown verifies that Submit returns ErrExecutorClosed once
// Shutdown has been called.
func TestSubmitAfterShutdown(t *testing.T) {
	be := NewBoundedExecutor(2, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := be.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}

	err := be.Submit(context.Background(), func(_ context.Context) {})
	if !errors.Is(err, execpkg.ErrExecutorClosed) {
		t.Errorf("expected ErrExecutorClosed, got %v", err)
	}
}

// TestShutdownIdempotent verifies that calling Shutdown more than once does
// not panic or return an error.
func TestShutdownIdempotent(t *testing.T) {
	be := NewBoundedExecutor(2, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 3; i++ {
		if err := be.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown call %d: unexpected error: %v", i+1, err)
		}
	}
}

// TestConcurrentSubmitShutdown is the race-detector test for the fix.
// It hammers Submit and Shutdown from multiple goroutines simultaneously.
// Run with: go test -race ./pkg/flow/exec/...
func TestConcurrentSubmitShutdown(t *testing.T) {
	const workers = 8
	const submitters = 16

	be := NewBoundedExecutor(workers, workers*4)

	var wg sync.WaitGroup
	var panics atomic.Int64

	// Start submitters that fire tasks as fast as possible.
	for i := 0; i < submitters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// Any panic here is a bug (e.g. send on closed channel).
					panics.Add(1)
				}
			}()
			for j := 0; j < 200; j++ {
				_ = be.Submit(context.Background(), func(_ context.Context) {
					// simulate tiny work
					time.Sleep(time.Microsecond)
				})
			}
		}()
	}

	// Give submitters a small head-start, then shut down.
	time.Sleep(2 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := be.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}

	wg.Wait()

	if n := panics.Load(); n > 0 {
		t.Errorf("detected %d panic(s) in submitters — likely send on closed channel", n)
	}
}

// TestShutdownContextCancel verifies that Shutdown respects a canceled context
// and returns ctx.Err() while long-running tasks are still in flight.
func TestShutdownContextCancel(t *testing.T) {
	be := NewBoundedExecutor(1, 1)

	// Submit a task that will block until we tell it to stop.
	unblock := make(chan struct{})
	_ = be.Submit(context.Background(), func(_ context.Context) {
		<-unblock
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := be.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	// Unblock the stuck task so the worker goroutine can exit cleanly.
	close(unblock)
}
