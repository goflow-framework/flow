package api

import (
	"net/http"

	flowpkg "github.com/undiegomejia/flow/pkg/flow"
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
	Redirect(code int, url string)
}

// Controller is a marker interface for controller implementations. Concrete
// controllers typically embed a BaseController provided by the framework.
type Controller interface{}

// BaseController provides a minimal contract for controllers that want to
// access the App (for services) and lightweight helpers. Implementations in
// pkg/flow will provide concrete helpers; the API package keeps the
// contract minimal.
type BaseController interface {
	App() *flowpkg.App
}
