package api

import (
	"net/http"
)

// Context is the minimal subset of the framework request context used by
// controllers. Using an interface here allows controllers to be tested and
// mocked without requiring full App wiring.
type Context interface {
	// Request exposes the underlying *http.Request
	Request() *http.Request
	// Writer returns the ResponseWriter to write responses
	Writer() http.ResponseWriter
	// Param reads a path parameter
	Param(name string) string
	// Render delegates to the ViewManager to render templates
	Render(name string, data interface{}) error
	// JSON writes a JSON response
	JSON(code int, v interface{}) error
	// Redirect sends an HTTP redirect
	Redirect(url string, code int)
	// BindForm decodes form-encoded request body into dst (pointer to struct).
	// Uses `form:"name"` struct tags; falls back to lowercase field names.
	BindForm(dst interface{}) error
	// BindQuery decodes URL query parameters into dst (pointer to struct).
	// Uses the same `form:"name"` tag convention as BindForm.
	BindQuery(dst interface{}) error
	// Validate runs struct-level validation on dst using the configured
	// validator. Returns validator.ValidationErrors on failure.
	Validate(dst interface{}) error
}

// AppLogger is a minimal logger interface that AppContext implementations may
// return. It matches the subset of log.Logger that the framework exposes.
// Kept here for use in future AppContext extensions.
type AppLogger interface {
	Printf(format string, v ...interface{})
}

// AppContext is the minimal interface that controllers use to reach framework
// services. It deliberately avoids importing pkg/flow so that the api package
// stays import-cycle-free.
//
// Note: Logger() is intentionally omitted from this interface because
// pkg/flow.App.Logger() returns a pkg/flow-local type. Use GetService to
// retrieve a logger from the service registry, or add a Logger() method once
// pkg/flow adopts api.AppLogger as its Logger return type.
type AppContext interface {
	GetService(name string) (interface{}, bool)
}

// Controller is a marker interface for controller implementations. Concrete
// controllers typically embed a BaseController provided by the framework.
type Controller interface{}

// BaseController provides a minimal contract for controllers that want to
// access the App (for services) and lightweight helpers. Implementations in
// pkg/flow will provide concrete helpers; the API package keeps the
// contract minimal.
type BaseController interface {
	App() AppContext
}
