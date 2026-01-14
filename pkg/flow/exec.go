package flow

import (
    "context"
    "errors"
    "sync/atomic"
    "time"
)

// Executor is an abstraction for submitting background work. Implementations
// may choose to bound concurrency and queue size.
type Executor interface {
    // Submit schedules fn to run with the provided context. It returns an
    // error if the executor is closed or the task cannot be accepted.
    Submit(ctx context.Context, fn func(context.Context)) error
    // Shutdown gracefully stops accepting new tasks and waits for running
    // tasks to complete until ctx is done. Implementations should return
    // nil if shutdown completes successfully or ctx.Err() otherwise.
    Shutdown(ctx context.Context) error
}

// ErrExecutorClosed is returned when submitting to a closed executor.
var ErrExecutorClosed = errors.New("executor: closed")

// BoundedExecutor is a simple bounded worker pool with a buffered task queue.
type BoundedExecutor struct {
    tasks     chan task
    stopOnce  atomic.Bool
    closed    chan struct{}
    inflight  int64 // number of currently running tasks
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
        return ErrExecutorClosed
    default:
    }
    t := task{ctx: ctx, fn: fn}
    select {
    case be.tasks <- t:
        return nil
    case <-be.closed:
        return ErrExecutorClosed
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
