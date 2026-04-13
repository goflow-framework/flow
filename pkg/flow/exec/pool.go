package flowexec

import (
	"context"
	"sync"

	execpkg "github.com/undiegomejia/flow/pkg/exec"
)

// BoundedExecutor is a simple bounded worker pool with a buffered task queue.
// It implements the shared exec.Executor interface.
//
// Concurrency safety
//
//   - Submit increments wg before enqueuing the task.  Workers decrement wg
//     after the task function returns.  This means wg.Wait() in Shutdown
//     cannot return zero prematurely: the count is raised by the producer
//     (Submit) before the task is visible to any consumer (worker).
//
//   - Shutdown signals workers to exit by closing be.closed (once, via
//     sync.Once) and then draining the tasks channel in a separate goroutine
//     so that workers that are blocked on a full queue can still exit.
//     be.tasks is intentionally NOT closed; workers exit via the be.closed
//     signal, which avoids the send-on-closed-channel race entirely.
//
//   - Submit holds be.mu while enqueuing so that closing be.closed and
//     enqueuing tasks are mutually exclusive.  Once be.closed is closed,
//     Submit returns ErrExecutorClosed immediately.
type BoundedExecutor struct {
	tasks    chan task
	stopOnce sync.Once
	closed   chan struct{}
	wg       sync.WaitGroup // counts tasks submitted but not yet completed
	mu       sync.Mutex     // guards the closed check + wg.Add + enqueue atomically
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
	for {
		select {
		case <-be.closed:
			return
		case t, ok := <-be.tasks:
			if !ok {
				return
			}
			t.fn(t.ctx)
			be.wg.Done()
		}
	}
}

// Submit enqueues a task or returns ErrExecutorClosed when the executor is
// shutting down. If the tasks channel buffer is full this will block until
// space is available or the context is canceled.
//
// wg.Add is called inside the mu lock, before the task is visible to any
// worker, so wg.Wait() in Shutdown will never see a false zero.
func (be *BoundedExecutor) Submit(ctx context.Context, fn func(context.Context)) error {
	// Fast path (no lock): check closed channel.
	select {
	case <-be.closed:
		return execpkg.ErrExecutorClosed
	default:
	}

	// Slow path: hold mu so that (closed-check + wg.Add + enqueue) are
	// atomic with respect to Shutdown's (mu.Lock + close(closed)).
	be.mu.Lock()
	select {
	case <-be.closed:
		be.mu.Unlock()
		return execpkg.ErrExecutorClosed
	default:
	}
	be.wg.Add(1)
	be.mu.Unlock()

	t := task{ctx: ctx, fn: fn}
	select {
	case be.tasks <- t:
		return nil
	case <-be.closed:
		// Executor shut down while we were waiting for queue space.
		// Roll back the wg.Add so Wait() doesn't block forever.
		be.wg.Done()
		return execpkg.ErrExecutorClosed
	case <-ctx.Done():
		be.wg.Done()
		return ctx.Err()
	}
}

// Shutdown signals the executor to stop accepting new tasks, waits for all
// in-flight tasks to complete, and then returns nil. If ctx is canceled before
// all tasks finish, ctx.Err() is returned; workers are NOT forcibly killed —
// they will finish their current task before exiting.
//
// Shutdown is idempotent; calling it more than once is safe.
func (be *BoundedExecutor) Shutdown(ctx context.Context) error {
	be.stopOnce.Do(func() {
		// Hold mu while closing be.closed so that concurrent Submit calls
		// either see the closed channel (and return ErrExecutorClosed) or
		// complete their wg.Add before we call wg.Wait below.
		be.mu.Lock()
		close(be.closed)
		be.mu.Unlock()
	})

	// Drain any tasks that were enqueued but whose worker is now blocked on
	// be.closed instead of be.tasks.  This prevents wg.Wait from blocking
	// if tasks are sitting in the buffered channel with no worker to consume
	// them (because all workers exited via be.closed).
	go func() {
		for {
			select {
			case t, ok := <-be.tasks:
				if !ok {
					return
				}
				t.fn(t.ctx)
				be.wg.Done()
			default:
				return
			}
		}
	}()

	// Wait for all in-flight tasks to finish, but respect ctx cancellation.
	done := make(chan struct{})
	go func() {
		be.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
