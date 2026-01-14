package flow

import (
	"context"
	"fmt"
	"sync"
)

// RequestGroup provides a small structured-concurrency helper for request-
// scoped goroutines. Spawn goroutines with Go and call Wait to block until
// all spawned goroutines complete. If any goroutine returns a non-nil error
// the group cancels the underlying context to signal other goroutines to
// stop and the error(s) are returned from Wait.
type RequestGroup struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	errs   []error
}

// NewRequestGroup returns a RequestGroup rooted at parent. If parent is nil,
// context.Background() is used.
func NewRequestGroup(parent context.Context) *RequestGroup {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &RequestGroup{ctx: ctx, cancel: cancel}
}

// Context returns the request-scoped context for goroutines spawned by the
// group. Callers should use this context to observe cancellation when other
// goroutines fail or when Wait completes.
func (g *RequestGroup) Context() context.Context { return g.ctx }

// Go runs fn in a new goroutine. If fn returns a non-nil error the group's
// context is cancelled and the error will be reported by Wait.
func (g *RequestGroup) Go(fn func(ctx context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(g.ctx); err != nil {
			g.mu.Lock()
			g.errs = append(g.errs, err)
			g.mu.Unlock()
			// cancel other goroutines
			g.cancel()
		}
	}()
}

// Wait blocks until all spawned goroutines have completed. If any goroutine
// returned an error, Wait returns either the single error or an aggregated
// error describing the number of failures.
func (g *RequestGroup) Wait() error {
	g.wg.Wait()
	// ensure cancel is called to release resources; safe to call multiple
	// times.
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errs) == 0 {
		return nil
	}
	if len(g.errs) == 1 {
		return g.errs[0]
	}
	return fmt.Errorf("%d errors: %v", len(g.errs), g.errs)
}
