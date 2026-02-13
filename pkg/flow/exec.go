package flow

import (
	flowexec "github.com/undiegomejia/flow/pkg/flow/exec"
)

// BoundedExecutor is a type alias for the implementation in
// pkg/flow/exec so callers that previously referenced the type in the
// flow package continue to compile. The actual implementation lives in
// pkg/flow/exec/pool.go.
type BoundedExecutor = flowexec.BoundedExecutor

// NewBoundedExecutor delegates to the package-local implementation so
// code that calls NewBoundedExecutor from package flow keeps working.
func NewBoundedExecutor(n, queueSize int) *BoundedExecutor {
	be := flowexec.NewBoundedExecutor(n, queueSize)
	return be
}
