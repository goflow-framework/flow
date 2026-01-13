// Package router implements a small, dependency-free HTTP router used
// internally by the Flow framework. It's intentionally simple and focused
// on the framework's conventions: RESTful resource routes, path parameters,
// explicit context passing, and no global state.
//
// Design goals:
// - Use net/http primitives
// - Explicit request context for params
// - Small, testable matching algorithm (segment-based)
// - Provide a Rails-like `Resources` helper for RESTful routes
//
// This package is internal to the framework; it purposely avoids exposing
// anything that would encourage reflection or magic.
package router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// ctxParamsKey is the context key used to store path parameters on requests.
type ctxParamsKey struct{}

// ParamsFromContext returns the route parameters stored on the request's
// context. If none are present an empty map is returned.
func ParamsFromContext(ctx context.Context) map[string]string {
	if ctx == nil {
		return map[string]string{}
	}
	if v, ok := ctx.Value(ctxParamsKey{}).(map[string]string); ok && v != nil {
		return v
	}
	return map[string]string{}
}

// Param is a convenience helper to fetch a single path parameter by name.
// It returns an empty string when not present.
func Param(r *http.Request, name string) string {
	return ParamsFromContext(r.Context())[name]
}

// ResourceController defines the canonical methods that a resource-style
// controller should implement in order to be wired with Router.Resources.
// All methods are simple http.HandlerFunc signatures so concrete controllers
// can be small and composable.
type ResourceController interface {
	Index(http.ResponseWriter, *http.Request)
	New(http.ResponseWriter, *http.Request)
	Create(http.ResponseWriter, *http.Request)
	Show(http.ResponseWriter, *http.Request)
	Edit(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Destroy(http.ResponseWriter, *http.Request)
}

// route holds the compiled representation of a single route.
type Middleware func(http.Handler) http.Handler

type route struct {
	method     string
	pattern    string
	segments   []string // pattern split by '/'
	handler    http.HandlerFunc
	name       string
	middleware []Middleware
}

// Router is a simple HTTP router that supports path parameters using the
// colon prefix (e.g. /users/:id) and a small RESTful DSL.
type Router struct {
	routes []*route
	// NotFound handler can be customized. If nil, http.NotFound is used.
	NotFound http.Handler
	// MethodNotAllowed handler called when a path matches but method doesn't.
	MethodNotAllowed http.Handler
}

// New creates an empty Router.
func New() *Router {
	return &Router{}
}

// Handle registers a handler for method and pattern.
// Pattern must start with '/'. Parameter segments start with ':' and match a
// single path segment.
func (r *Router) Handle(method, pattern string, h http.HandlerFunc) {
	if !strings.HasPrefix(pattern, "/") {
		panic("router: pattern must begin with '/'")
	}
	segs := splitPath(pattern)
	rt := &route{method: strings.ToUpper(method), pattern: pattern, segments: segs, handler: h}
	r.routes = append(r.routes, rt)
}

// HandleWith allows attaching per-route middleware for this route.
func (r *Router) HandleWith(method, pattern string, h http.HandlerFunc, mws ...Middleware) {
	if !strings.HasPrefix(pattern, "/") {
		panic("router: pattern must begin with '/'")
	}
	segs := splitPath(pattern)
	rt := &route{method: strings.ToUpper(method), pattern: pattern, segments: segs, handler: h, middleware: mws}
	r.routes = append(r.routes, rt)
}

// HandleNamed registers a named route. If the name is already in use the function panics.
func (r *Router) HandleNamed(name, method, pattern string, h http.HandlerFunc) {
	if name == "" {
		panic("router: route name cannot be empty")
	}
	// ensure uniqueness
	for _, existing := range r.routes {
		if existing.name == name {
			panic(fmt.Sprintf("router: duplicate route name %s", name))
		}
	}
	if !strings.HasPrefix(pattern, "/") {
		panic("router: pattern must begin with '/'")
	}
	segs := splitPath(pattern)
	rt := &route{method: strings.ToUpper(method), pattern: pattern, segments: segs, handler: h, name: name}
	r.routes = append(r.routes, rt)
}

// HandleNamedWith registers a named route with per-route middleware.
func (r *Router) HandleNamedWith(name, method, pattern string, h http.HandlerFunc, mws ...Middleware) {
	if name == "" {
		panic("router: route name cannot be empty")
	}
	for _, existing := range r.routes {
		if existing.name == name {
			panic(fmt.Sprintf("router: duplicate route name %s", name))
		}
	}
	if !strings.HasPrefix(pattern, "/") {
		panic("router: pattern must begin with '/'")
	}
	segs := splitPath(pattern)
	rt := &route{method: strings.ToUpper(method), pattern: pattern, segments: segs, handler: h, name: name, middleware: mws}
	r.routes = append(r.routes, rt)
}

// Convenience sugar: GetNamed, PostNamed, PutNamed, PatchNamed, DeleteNamed
func (r *Router) GetNamed(name, pattern string, h http.HandlerFunc) {
	r.HandleNamed(name, "GET", pattern, h)
}
func (r *Router) PostNamed(name, pattern string, h http.HandlerFunc) {
	r.HandleNamed(name, "POST", pattern, h)
}
func (r *Router) PutNamed(name, pattern string, h http.HandlerFunc) {
	r.HandleNamed(name, "PUT", pattern, h)
}
func (r *Router) PatchNamed(name, pattern string, h http.HandlerFunc) {
	r.HandleNamed(name, "PATCH", pattern, h)
}
func (r *Router) DeleteNamed(name, pattern string, h http.HandlerFunc) {
	r.HandleNamed(name, "DELETE", pattern, h)
}

// Convenience sugar with per-route middleware
func (r *Router) GetWith(pattern string, h http.HandlerFunc, mws ...Middleware) {
	r.HandleWith("GET", pattern, h, mws...)
}
func (r *Router) PostWith(pattern string, h http.HandlerFunc, mws ...Middleware) {
	r.HandleWith("POST", pattern, h, mws...)
}
func (r *Router) PutWith(pattern string, h http.HandlerFunc, mws ...Middleware) {
	r.HandleWith("PUT", pattern, h, mws...)
}
func (r *Router) PatchWith(pattern string, h http.HandlerFunc, mws ...Middleware) {
	r.HandleWith("PATCH", pattern, h, mws...)
}
func (r *Router) DeleteWith(pattern string, h http.HandlerFunc, mws ...Middleware) {
	r.HandleWith("DELETE", pattern, h, mws...)
}

// convenience methods
func (r *Router) Get(p string, h http.HandlerFunc)    { r.Handle("GET", p, h) }
func (r *Router) Post(p string, h http.HandlerFunc)   { r.Handle("POST", p, h) }
func (r *Router) Put(p string, h http.HandlerFunc)    { r.Handle("PUT", p, h) }
func (r *Router) Patch(p string, h http.HandlerFunc)  { r.Handle("PATCH", p, h) }
func (r *Router) Delete(p string, h http.HandlerFunc) { r.Handle("DELETE", p, h) }

// Resources wires a ResourceController to standard RESTful routes using the
// given base path (e.g. "users"). The base should not contain leading or
// trailing slashes; Router will construct the conventional paths.
func (r *Router) Resources(base string, c ResourceController) error {
	if base == "" {
		return fmt.Errorf("router: Resources base cannot be empty")
	}
	base = strings.Trim(base, "/")

	// index, new, create
	r.GetNamed(fmt.Sprintf("%s_index", base), fmt.Sprintf("/%s", base), c.Index)
	r.GetNamed(fmt.Sprintf("%s_new", base), fmt.Sprintf("/%s/new", base), c.New)
	r.PostNamed(fmt.Sprintf("%s_create", base), fmt.Sprintf("/%s", base), c.Create)

	// member routes: show, edit, update, destroy
	member := fmt.Sprintf("/%s/:id", base)
	r.GetNamed(fmt.Sprintf("%s_show", base), member, c.Show)
	r.GetNamed(fmt.Sprintf("%s_edit", base), fmt.Sprintf("/%s/:id/edit", base), c.Edit)
	r.PutNamed(fmt.Sprintf("%s_update", base), member, c.Update)
	r.PatchNamed(fmt.Sprintf("%s_patch", base), member, c.Update)
	r.DeleteNamed(fmt.Sprintf("%s_destroy", base), member, c.Destroy)

	return nil
}

// UseParamsPool controls whether parameter maps are reused from a pool.
// Tests/benchmarks can toggle this to compare behavior.
var UseParamsPool = true

var paramsPool sync.Pool

func getParams() map[string]string {
	if !UseParamsPool {
		return make(map[string]string)
	}
	if v := paramsPool.Get(); v != nil {
		m := v.(map[string]string)
		// clear
		for k := range m {
			delete(m, k)
		}
		return m
	}
	return make(map[string]string)
}

func putParams(m map[string]string) {
	if !UseParamsPool {
		return
	}
	// clear before putting back
	for k := range m {
		delete(m, k)
	}
	paramsPool.Put(m)
}

// ServeHTTP implements http.Handler. It finds the first matching route
// (in registration order), injects params into the request context, and
// invokes the handler. If no route matches, NotFound is called. If a path
// matches but the method does not, MethodNotAllowed is called.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := normalizePath(req.URL.Path)
	var methodMismatch bool

	for _, rt := range r.routes {
		ok, params := matchRoute(rt.segments, path)
		if !ok {
			continue
		}
		if rt.method != req.Method {
			methodMismatch = true
			continue
		}

		// inject params into context
		ctx := context.WithValue(req.Context(), ctxParamsKey{}, params)
		// build handler with route middleware (first registered is outer-most)
		var final http.Handler = http.HandlerFunc(rt.handler)
		for i := len(rt.middleware) - 1; i >= 0; i-- {
			final = rt.middleware[i](final)
		}
		// If params were allocated from the pool we must return them after
		// the handler finished. We only wrap when params is non-nil and
		// non-empty to avoid extra allocations on routes without params.
		if params != nil && len(params) > 0 {
			orig := final
			final = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				orig.ServeHTTP(w, r)
				putParams(params)
			})
		}
		final.ServeHTTP(w, req.WithContext(ctx))
		return
	}

	if methodMismatch {
		if r.MethodNotAllowed != nil {
			r.MethodNotAllowed.ServeHTTP(w, req)
			return
		}
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if r.NotFound != nil {
		r.NotFound.ServeHTTP(w, req)
		return
	}
	http.NotFound(w, req)
}

// splitPath splits a pattern into segments, preserving parameter segments.
// Example: "/users/:id/edit" -> ["users", ":id", "edit"]
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	parts := strings.Split(p, "/")
	return parts
}

// URL builds a path for a named route by substituting params into the
// named route's pattern. Returns an error if the name is unknown or if a
// required param is missing. Param values are path-escaped.
func (r *Router) URL(name string, params map[string]string) (string, error) {
	for _, rt := range r.routes {
		if rt.name == name {
			if len(rt.segments) == 0 {
				return "/", nil
			}
			parts := make([]string, 0, len(rt.segments))
			for _, s := range rt.segments {
				if strings.HasPrefix(s, ":") {
					key := strings.TrimPrefix(s, ":")
					v, ok := params[key]
					if !ok {
						return "", fmt.Errorf("router: missing param %s for route %s", key, name)
					}
					parts = append(parts, url.PathEscape(v))
					continue
				}
				parts = append(parts, s)
			}
			return "/" + strings.Join(parts, "/"), nil
		}
	}
	return "", fmt.Errorf("router: unknown route %s", name)
}

// normalizePath prepares an incoming request path for matching.
// It removes a trailing slash unless the path is just "/".
func normalizePath(p string) string {
	if p == "/" {
		return p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// matchRoute attempts to match the candidate path to the route segments.
// Returns ok and a map of parameters when matched.
func matchRoute(segs []string, path string) (bool, map[string]string) {
	// handle root
	if len(segs) == 0 {
		return path == "/", map[string]string{}
	}

	// trim leading and trailing slashes without allocating
	start := 0
	end := len(path)
	if end > 0 && path[0] == '/' {
		start = 1
	}
	if end > 0 && path[end-1] == '/' {
		end--
	}
	if start >= end {
		return false, nil
	}

	// iterate over segments in-place to avoid allocating a []string via strings.Split
	var params map[string]string
	pstart := start
	segIndex := 0
	for segIndex < len(segs) {
		// find end of this part
		pend := pstart
		for pend < end && path[pend] != '/' {
			pend++
		}
		part := path[pstart:pend]

		s := segs[segIndex]
		if s == "" {
			if part != "" {
				return false, nil
			}
			// advance
			segIndex++
			pstart = pend + 1
			continue
		}
		if strings.HasPrefix(s, ":") {
			name := strings.TrimPrefix(s, ":")
			if name == "" {
				return false, nil
			}
			if params == nil {
				params = getParams()
			}
			params[name] = part
		} else {
			if s != part {
				return false, nil
			}
		}
		segIndex++
		pstart = pend + 1
		// if we've consumed the path but there are still segments left, fail
		if pstart-1 > end && segIndex < len(segs) {
			return false, nil
		}
	}

	// ensure there are no leftover parts in path
	if pstart <= end {
		// if there are non-empty trailing parts, fail
		if pstart <= end-1 {
			return false, nil
		}
	}

	if params == nil {
		return true, map[string]string{}
	}
	return true, params
}

// TODO: Consider adding support for named route lookup, middleware per-route
// and wildcard segments ("*path") should the framework require them later.
