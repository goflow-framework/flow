// Package flow: public router adapter.
//
// This file exposes a framework-friendly Router that wraps the internal
// routing engine. It keeps the public API small: users of the framework
// should import pkg/flow and use flow.NewRouter(app) to register routes
// and resources using idiomatic types (flow.Resource, flow.Controller).
package flow

import (
	"fmt"
	"net/http"

	routerpkg "github.com/undiegomejia/flow/internal/router"
)

// Router is the public wrapper around internal/router.Router. It accepts
// framework Resource implementations and controller handlers and exposes
// a small, testable surface.
type Router struct {
	inner *routerpkg.Router
	app   *App
}

// NewRouter constructs a Router bound to the provided App. App may be nil
// for tests, but Resource adapters that need App will require a non-nil
// App to function correctly.
func NewRouter(app *App) *Router {
	return &Router{inner: routerpkg.New(), app: app}
}

// Get registers a GET handler that accepts a *flow.Context for the given pattern.
// The provided handler will be adapted into an http.HandlerFunc using the
// Router's App reference (may be nil for tests).
func (r *Router) Get(pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.Get(pattern, wrapped)
}

// Post registers a POST handler that accepts a *flow.Context.
func (r *Router) Post(pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.Post(pattern, wrapped)
}

// Put registers a PUT handler that accepts a *flow.Context.
func (r *Router) Put(pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.Put(pattern, wrapped)
}

// Patch registers a PATCH handler that accepts a *flow.Context.
func (r *Router) Patch(pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.Patch(pattern, wrapped)
}

// Delete registers a DELETE handler that accepts a *flow.Context.
func (r *Router) Delete(pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.Delete(pattern, wrapped)
}

// With variants accept framework Middleware and attach them to the route.
// The provided Middleware are applied in registration order (first is outer-most).
func (r *Router) GetWith(pattern string, h func(*Context), mws ...Middleware) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	// convert flow.Middleware to routerpkg.Middleware
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	r.inner.GetWith(pattern, wrapped, conv...)
}

func (r *Router) PostWith(pattern string, h func(*Context), mws ...Middleware) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	r.inner.PostWith(pattern, wrapped, conv...)
}

func (r *Router) PutWith(pattern string, h func(*Context), mws ...Middleware) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	r.inner.PutWith(pattern, wrapped, conv...)
}

func (r *Router) PatchWith(pattern string, h func(*Context), mws ...Middleware) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	r.inner.PatchWith(pattern, wrapped, conv...)
}

func (r *Router) DeleteWith(pattern string, h func(*Context), mws ...Middleware) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	r.inner.DeleteWith(pattern, wrapped, conv...)
}

// Resources wires a flow.Resource into RESTful routes using the conventional
// path base. It uses MakeResourceAdapter to adapt the Resource to the
// internal router.ResourceController.
func (r *Router) Resources(base string, res Resource) error {
	if r.app == nil {
		return fmt.Errorf("router: cannot register resources without an App; provide an App to NewRouter")
	}
	return r.inner.Resources(base, MakeResourceAdapter(r.app, res))
}

// ServeHTTP forwards to the internal router's ServeHTTP implementation.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.inner.ServeHTTP(w, req)
}

// Handler returns the underlying http.Handler so the Router can be used
// directly with net/http servers.
func (r *Router) Handler() http.Handler { return r.inner }
