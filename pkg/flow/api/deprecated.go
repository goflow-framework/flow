package api

// The LegacyRouter, LegacyViewManager and LegacyContext aliases that
// previously lived here have been removed. They re-introduced an import of
// pkg/flow into pkg/flow/api, creating an import cycle.
//
// Migration guide:
//   - Replace LegacyRouter      with api.Router      (interface, this package)
//   - Replace LegacyViewManager with api.ViewManager  (interface, this package)
//   - Replace LegacyContext      with api.Context      (interface, this package)
