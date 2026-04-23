package flowexec

import (
	"context"
	"sync"

	execpkg "github.com/goflow-framework/flow/pkg/exec"
)

// BoundedExecutor is a simple bounded worker pool with a buffered task queue.
// It implements the shared exec.Executor interface.
//
// Concurrency safety
//
//   - Submit increments be.wg (task counter) and be.senders (in-flight sender
//     counter) inside be.mu, before the task is visible to any worker.
//     be.wg counts tasks that have been submitted but not yet completed.
//     be.senders counts goroutines that have incremented be.wg but have not
//     yet either deposited the task in be.tasks or rolled back be.wg.
//
//   - Shutdown closes be.closed (under be.mu), then waits for be.senders to
//     reach zero — at that point no goroutine can add new tasks to be.tasks,
//     so closing be.tasks is safe.  Workers use "for t := range be.tasks" and
//     exit naturally once the channel is drained and closed.
//
//   - Submit holds be.mu while doing (closed-check + be.wg.Add +
//     be.senders.Add) so the close in Shutdown is mutually exclusive with
//     those operations.  Once be.closed is closed, Submit returns
//     ErrExecutorClosed immediately without incrementing either counter.
type BoundedExecutor struct {
	tasks    chan task
	stopOnce sync.Once
	closed   chan struct{}
	wg       sync.WaitGroup // counts tasks submitted but not yet completed
	senders  sync.WaitGroup // counts goroutines between wg.Add and channel send/rollback
	mu       sync.Mutex     // guards closed-check + wg.Add + senders.Add atomically
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
		t.fn(t.ctx)
		be.wg.Done()
	}
}

// Submit enqueues a task or returns ErrExecutorClosed when the executor is
// shutting down. If the tasks channel buffer is full this will block until
// space is available or the context is canceled.
//
// wg.Add and senders.Add are called inside the mu lock, before the task is
// visible to any worker, so wg.Wait() in Shutdown will never see a false zero.
func (be *BoundedExecutor) Submit(ctx context.Context, fn func(context.Context)) error {
	// Fast path (no lock): check closed channel.
	select {
	case <-be.closed:
		return execpkg.ErrExecutorClosed
	default:
	}

	// Slow path: hold mu so that (closed-check + wg.Add + senders.Add) are
	// atomic with respect to Shutdown's (mu.Lock + close(closed)).
	be.mu.Lock()
	select {
	case <-be.closed:
		be.mu.Unlock()
		return execpkg.ErrExecutorClosed
	default:
	}
	be.wg.Add(1)
	be.senders.Add(1)
	be.mu.Unlock()

	// At this point we are a "sender": we've incremented wg and senders, and
	// we must either deposit the task or roll back before calling senders.Done.
	t := task{ctx: ctx, fn: fn}
	select {
	case be.tasks <- t:
		// Task is now in the queue; workers will run it and call wg.Done.
		be.senders.Done()
		return nil
	case <-be.closed:
		// Executor shut down while we were waiting for queue space.
		// Roll back wg so Shutdown's wg.Wait() doesn't block forever.
		be.wg.Done()
		be.senders.Done()
		return execpkg.ErrExecutorClosed
	case <-ctx.Done():
		be.wg.Done()
		be.senders.Done()
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
		// Close be.closed under mu so that Submit's (closed-check + wg.Add +
		// senders.Add) is atomic with this close.  After mu is released, no new
		// sender can start; existing senders will either deposit or roll back.
		be.mu.Lock()
		close(be.closed)
		be.mu.Unlock()

		// Wait until all in-flight senders have either put their task in the
		// channel or rolled back their wg.Add.  Only then is it safe to close
		// be.tasks — no goroutine can send to it after this point.
		be.senders.Wait()
		close(be.tasks)
	})

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
