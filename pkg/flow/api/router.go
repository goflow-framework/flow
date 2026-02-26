// Package api contains stable interface definitions for Flow's public API.
//
// These interfaces define the contracts for the Router and routing-related
// types. They are intentionally small and declarative so other packages and
// (eventually) third-party adapters can implement or mock them without
// depending on concrete implementations.
package api

import (
	"net/http"

	flowpkg "github.com/undiegomejia/flow/pkg/flow"
)

// ContextHandler is the canonical handler signature used by framework
// controllers and routes. It accepts a framework Context which carries the
// request, response writer and helpers.
type ContextHandler func(*flowpkg.Context)

// Middleware wraps an http.Handler and returns a new handler. Middlewares
// are applied in registration order (first registered is outer-most).
type Middleware func(http.Handler) http.Handler

// Resource is the minimal interface a resource-style controller should
// implement to be wired via Router.Resources.
type Resource interface {
	Index(*flowpkg.Context)
	New(*flowpkg.Context)
	Create(*flowpkg.Context)
	Show(*flowpkg.Context)
	Edit(*flowpkg.Context)
	Update(*flowpkg.Context)
	Destroy(*flowpkg.Context)
}

// RouteGroup represents a group of routes sharing a common prefix and
// middleware. Implementations should apply group middleware outer-most.
type RouteGroup interface {
	Get(pattern string, h ContextHandler)
	Post(pattern string, h ContextHandler)
	Put(pattern string, h ContextHandler)
	Patch(pattern string, h ContextHandler)
	Delete(pattern string, h ContextHandler)

	GetWith(pattern string, h ContextHandler, mws ...Middleware)
	PostWith(pattern string, h ContextHandler, mws ...Middleware)
	PutWith(pattern string, h ContextHandler, mws ...Middleware)
	PatchWith(pattern string, h ContextHandler, mws ...Middleware)
	DeleteWith(pattern string, h ContextHandler, mws ...Middleware)

	GetNamed(name, pattern string, h ContextHandler)
	PostNamed(name, pattern string, h ContextHandler)
	PutNamed(name, pattern string, h ContextHandler)
	PatchNamed(name, pattern string, h ContextHandler)
	DeleteNamed(name, pattern string, h ContextHandler)
}

// Router is the public routing contract used by framework consumers. It
// focuses on registering Context-based handlers and basic URL generation.
type Router interface {
	// Handler returns a net/http handler suitable for use with servers.
	Handler() http.Handler

	// Basic verbs using ContextHandler.
	Get(pattern string, h ContextHandler)
	Post(pattern string, h ContextHandler)
	Put(pattern string, h ContextHandler)
	Patch(pattern string, h ContextHandler)
	Delete(pattern string, h ContextHandler)

	// Named route registration (useful for URL generation)
	GetNamed(name, pattern string, h ContextHandler)
	PostNamed(name, pattern string, h ContextHandler)
	PutNamed(name, pattern string, h ContextHandler)
	PatchNamed(name, pattern string, h ContextHandler)
	DeleteNamed(name, pattern string, h ContextHandler)

	// Resources wires a Resource into conventional RESTful routes.
	Resources(base string, res Resource) error

	// Group creates a RouteGroup with a prefix and optional middleware.
	Group(prefix string, mws ...Middleware) RouteGroup

	// URL builds a path for a named route using the provided parameters.
	// Returns an error when the name is unknown or required params are missing.
	URL(name string, params map[string]string) (string, error)
}
