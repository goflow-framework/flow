package flowexec

import (
	"context"
	"sync/atomic"
	"time"

	execpkg "github.com/undiegomejia/flow/pkg/exec"
)

// BoundedExecutor is a simple bounded worker pool with a buffered task queue.
// It implements the shared exec.Executor interface.
type BoundedExecutor struct {
	tasks    chan task
	stopOnce atomic.Bool
	closed   chan struct{}
	inflight int64 // number of currently running tasks
}

type task struct {
	ctx context.Context
	fn  func(context.Context)
}

// NewBoundedExecutor creates an executor with n workers and a task queue of
// queueSize. If queueSize is zero it defaults to n.
func NewBoundedExecutor(n, queueSize int) *BoundedExecutor {
	if n <= 0 {
		n = 1
	}
	if queueSize <= 0 {
		queueSize = n
	}
	be := &BoundedExecutor{
		tasks:  make(chan task, queueSize),
		closed: make(chan struct{}),
	}
	for i := 0; i < n; i++ {
		go be.worker()
	}
	return be
}

func (be *BoundedExecutor) worker() {
	for t := range be.tasks {
		atomic.AddInt64(&be.inflight, 1)
		t.fn(t.ctx)
		atomic.AddInt64(&be.inflight, -1)
	}
}

// Submit enqueues a task or returns ErrExecutorClosed when the executor is
// shutting down. If the tasks channel buffer is full this will block until
// space is available or the context is canceled.
func (be *BoundedExecutor) Submit(ctx context.Context, fn func(context.Context)) error {
	// if closed, return right away
	select {
	case <-be.closed:
		return execpkg.ErrExecutorClosed
	default:
	}
	t := task{ctx: ctx, fn: fn}
	select {
	case be.tasks <- t:
		return nil
	case <-be.closed:
		return execpkg.ErrExecutorClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown closes the task channel and waits for in-flight tasks to finish.
func (be *BoundedExecutor) Shutdown(ctx context.Context) error {
	if be.stopOnce.CompareAndSwap(false, true) {
		close(be.closed)
		close(be.tasks)
	}
	// wait until inflight tasks drop to zero
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if atomic.LoadInt64(&be.inflight) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
