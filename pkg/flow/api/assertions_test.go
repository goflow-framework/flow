package api_test

// Compile-time assertions that pkg/flow's concrete types satisfy the api
// interfaces. If either concrete type drifts from an interface the build
// will fail here with a clear message.
//
// These live in an external test package (api_test) so they can import both
// pkg/flow/api and pkg/flow without creating an import cycle.

import (
	"github.com/undiegomejia/flow/pkg/flow"
	"github.com/undiegomejia/flow/pkg/flow/api"
)

// *flow.Context must satisfy api.Context.
var _ api.Context = (*flow.Context)(nil)

// *flow.App must satisfy api.AppContext.
var _ api.AppContext = (*flow.App)(nil)
