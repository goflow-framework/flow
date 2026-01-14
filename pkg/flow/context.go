package flow

import (
	"context"
	"net/http"
	"sync"

	routerpkg "github.com/undiegomejia/flow/internal/router"
)

type Context struct {
	// App is an optional reference to the running application. It is kept
	// as an interface to avoid tight coupling; controllers can use it to
	// access logger, config, or shared services.
	App *App

	// W is the response writer for the current request.
	W http.ResponseWriter

	// R is the incoming http request.
	R *http.Request

	// status stores the last status set via Status or one of the render
	// helpers. Zero means unset; helper methods will set sensible defaults.
	status int

	// rg is an optional per-request RequestGroup used to spawn request-
	// scoped goroutines. It is lazily initialized by RequestGroup().
	rg *RequestGroup
}

// NewContext constructs a Context. App may be nil for tests or simple
// handlers. It retrieves instances from an internal pool to reduce per-
// request allocations. Callers should return pooled Contexts with
// PutContext when the request is finished (framework adapters do this for
// you in the hot path).
func NewContext(app *App, w http.ResponseWriter, r *http.Request) *Context {
	if UseContextPool {
		if v := contextPool.Get(); v != nil {
			c := v.(*Context)
			c.App = app
			c.W = w
			c.R = r
			c.status = 0
			c.rg = nil
			return c
		}
	}
	return &Context{App: app, W: w, R: r}
}

var contextPool sync.Pool

// UseContextPool toggles whether NewContext/PutContext use the internal
// pool. Tests and benchmarks can set this to false to measure baseline
// behavior without pooling.
var UseContextPool = true

// PutContext returns a Context to the internal pool. Call this after the
// request has been processed to allow the instance to be reused. It clears
// references to avoid leaking request-local data.
func PutContext(c *Context) {
	if c == nil {
		return
	}
	// clear references
	c.App = nil
	c.W = nil
	c.R = nil
	c.status = 0
	if c.rg != nil {
		// Cancel any running RequestGroup goroutines; we don't wait here to
		// avoid blocking the hot path. Handlers that spawn goroutines should
		// call Wait themselves if they need to observe completion.
		c.rg.Cancel()
		c.rg = nil
	}
	if UseContextPool {
		contextPool.Put(c)
	}
}

// RequestGroup returns a request-scoped RequestGroup. It is lazily created
// and anchored to the request's context so deadlines/cancellations propagate.
func (c *Context) RequestGroup() *RequestGroup {
	if c.rg == nil {
		// Use the request context so cancellations and deadlines carry over.
		parent := context.Background()
		if c.R != nil {
			parent = c.R.Context()
		}
		c.rg = NewRequestGroup(parent)
	}
	return c.rg
}
