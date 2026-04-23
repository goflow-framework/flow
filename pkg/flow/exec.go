package flow

import (
	flowexec "github.com/goflow-framework/flow/pkg/flow/exec"
)

// NewBoundedExecutor is a compatibility shim that delegates to the
// implementation in pkg/flow/exec. Existing callers using flow.NewBoundedExecutor
// will continue to work; prefer using flow.WithBoundedExecutor or importing
// pkg/flow/exec when constructing executors directly.
func NewBoundedExecutor(n, queueSize int) *flowexec.BoundedExecutor {
	return flowexec.NewBoundedExecutor(n, queueSize)
}
