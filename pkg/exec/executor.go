package exec

import (
	"context"
	"errors"
)

// Executor is a small shared interface for submitting background work.
// It is intentionally minimal so multiple packages can depend on it without
// creating import cycles.
type Executor interface {
	Submit(ctx context.Context, fn func(context.Context)) error
	Shutdown(ctx context.Context) error
}

// ErrExecutorClosed is returned when submitting to a closed executor.
var ErrExecutorClosed = errors.New("executor: closed")
