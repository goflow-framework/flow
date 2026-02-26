package api

import (
	flowpkg "github.com/undiegomejia/flow/pkg/flow"
)

// Deprecated aliases provide a thin compatibility layer for users who want
// to reference legacy concrete types via the new api package. These are
// intentionally simple aliases that will be removed once the API package is
// fully adopted.

// LegacyRouter is an alias to the existing concrete Router type in
// pkg/flow. Use the Router interface in this package for new code.
type LegacyRouter = flowpkg.Router

// LegacyViewManager is an alias to the concrete ViewManager implementation.
type LegacyViewManager = flowpkg.ViewManager

// LegacyContext is an alias for the concrete context type used by
// controllers. Prefer the minimal api.Context interface for testing.
type LegacyContext = flowpkg.Context
