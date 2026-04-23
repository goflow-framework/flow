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

	routerpkg "github.com/goflow-framework/flow/internal/router"
)

// Router is the public wrapper around internal/router.Router. It accepts
// framework Resource implementations and controller handlers and exposes
// a small, testable surface.
type Router struct {
	inner *routerpkg.Router
	app   *App
}

// RouteGroup is the public wrapper around an internal router Group. It
// exposes the same registration helpers but accepts framework Context
// handlers and flow.Middleware.
type RouteGroup struct {
	inner *routerpkg.Group
	r     *Router
}

// NewRouter constructs a Router bound to the provided App. App may be nil
// for tests, but Resource adapters that need App will require a non-nil
// App to function correctly.
func NewRouter(app *App) *Router {
	return &Router{inner: routerpkg.New(), app: app}
}

// Group creates a new route group with the provided prefix and optional
// middleware. Group middleware is applied outer-most.
func (r *Router) Group(prefix string, mws ...Middleware) *RouteGroup {
	// convert middleware types
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	return &RouteGroup{inner: r.inner.Group(prefix, conv...), r: r}
}

// RouteGroup registration helpers adapt flow.Context handlers to http handlers
// and delegate to the internal group.
func (g *RouteGroup) wrap(h func(*Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(g.r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
}

func (g *RouteGroup) Get(pattern string, h func(*Context))  { g.inner.Get(g.join(pattern), g.wrap(h)) }
func (g *RouteGroup) Post(pattern string, h func(*Context)) { g.inner.Post(g.join(pattern), g.wrap(h)) }
func (g *RouteGroup) Put(pattern string, h func(*Context))  { g.inner.Put(g.join(pattern), g.wrap(h)) }
func (g *RouteGroup) Patch(pattern string, h func(*Context)) {
	g.inner.Patch(g.join(pattern), g.wrap(h))
}
func (g *RouteGroup) Delete(pattern string, h func(*Context)) {
	g.inner.Delete(g.join(pattern), g.wrap(h))
}

func (g *RouteGroup) GetWith(pattern string, h func(*Context), mws ...Middleware) {
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	g.inner.GetWith(g.join(pattern), g.wrap(h), conv...)
}
func (g *RouteGroup) PostWith(pattern string, h func(*Context), mws ...Middleware) {
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	g.inner.PostWith(g.join(pattern), g.wrap(h), conv...)
}
func (g *RouteGroup) PutWith(pattern string, h func(*Context), mws ...Middleware) {
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	g.inner.PutWith(g.join(pattern), g.wrap(h), conv...)
}
func (g *RouteGroup) PatchWith(pattern string, h func(*Context), mws ...Middleware) {
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	g.inner.PatchWith(g.join(pattern), g.wrap(h), conv...)
}
func (g *RouteGroup) DeleteWith(pattern string, h func(*Context), mws ...Middleware) {
	conv := make([]routerpkg.Middleware, 0, len(mws))
	for _, mw := range mws {
		conv = append(conv, routerpkg.Middleware(mw))
	}
	g.inner.DeleteWith(g.join(pattern), g.wrap(h), conv...)
}

func (g *RouteGroup) GetNamed(name, pattern string, h func(*Context)) {
	g.inner.GetNamed(name, g.join(pattern), g.wrap(h))
}
func (g *RouteGroup) PostNamed(name, pattern string, h func(*Context)) {
	g.inner.PostNamed(name, g.join(pattern), g.wrap(h))
}
func (g *RouteGroup) PutNamed(name, pattern string, h func(*Context)) {
	g.inner.PutNamed(name, g.join(pattern), g.wrap(h))
}
func (g *RouteGroup) PatchNamed(name, pattern string, h func(*Context)) {
	g.inner.PatchNamed(name, g.join(pattern), g.wrap(h))
}
func (g *RouteGroup) DeleteNamed(name, pattern string, h func(*Context)) {
	g.inner.DeleteNamed(name, g.join(pattern), g.wrap(h))
}

// join is a small helper to ensure patterns passed to the internal group are
// understood as relative (internal Group already manages prefixing) but we
// leave this to be explicit for now.
func (g *RouteGroup) join(p string) string { return p }

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

// URL builds a path for a named route by delegating to the internal router.
// It returns an error if the route is unknown or required params are missing.
func (r *Router) URL(name string, params map[string]string) (string, error) {
	return r.inner.URL(name, params)
}

// Named route registration helpers mirror the internal router API but accept
// framework Context handlers. These are useful for code that needs named
// routes for URL generation while still using flow.Context handlers.
func (r *Router) GetNamed(name, pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.GetNamed(name, pattern, wrapped)
}

func (r *Router) PostNamed(name, pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.PostNamed(name, pattern, wrapped)
}

func (r *Router) PutNamed(name, pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.PutNamed(name, pattern, wrapped)
}

func (r *Router) PatchNamed(name, pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.PatchNamed(name, pattern, wrapped)
}

func (r *Router) DeleteNamed(name, pattern string, h func(*Context)) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := NewContext(r.app, w, req)
		defer PutContext(ctx)
		h(ctx)
	}
	r.inner.DeleteNamed(name, pattern, wrapped)
}
