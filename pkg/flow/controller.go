// Package flow provides small, composable controller base types and
// adapters used by application code. Controllers in Flow should be
// lightweight structs that embed or compose this base type and implement
// action methods that accept a *flow.Context.
//
// This file implements:
//   - Controller: a small struct holding an App reference and helper methods
//   - Resource: an interface describing resource-style controller actions
//   - Adapter to convert a Resource implementation into the internal
//     router.ResourceController (which uses http.Handler signatures).
package flow

import (
	"fmt"
	"net/http"

	routerpkg "github.com/goflow-framework/flow/internal/router"
)

// Controller is a minimal base that application controllers can embed or
// compose. It holds a reference to the App so actions can access shared
// services (logger, DB connections, config, etc.).
type Controller struct {
	App *App
}

// NewController is a convenience constructor.
func NewController(app *App) *Controller { return &Controller{App: app} }

// WithContext constructs a *flow.Context for the current request. This is
// a thin adapter so controller actions can create contexts easily.
func (c *Controller) WithContext(w http.ResponseWriter, r *http.Request) *Context {
	return NewContext(c.App, w, r)
}

// Handler converts an action function that accepts *Context into an
// http.HandlerFunc usable with the standard library and internal router.
func (c *Controller) Handler(action func(*Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := NewContext(c.App, w, r)
		defer PutContext(ctx)
		action(ctx)
	}
}

// Render resolves and executes a template using the App's ViewManager.
// Template names follow the convention "users/show" (relative to the
// configured views directory). Returns an error if rendering fails.
func (c *Controller) Render(ctx *Context, name string, data interface{}) error {
	if c.App == nil || c.App.Views == nil {
		return fmt.Errorf("controller: view manager not configured")
	}
	return c.App.Views.Render(name, data, ctx)
}

// Resource defines the idiomatic controller methods for RESTful resources.
// Application controllers implementing resourceful behavior should implement
// these methods. This keeps controller implementations small and focused on
// request handling rather than HTTP plumbing.
type Resource interface {
	Index(*Context)
	New(*Context)
	Create(*Context)
	Show(*Context)
	Edit(*Context)
	Update(*Context)
	Destroy(*Context)
}

// resourceAdapter adapts a Resource (methods that accept *flow.Context)
// to the internal router.ResourceController which expects methods with
// (http.ResponseWriter, *http.Request) signatures.
type resourceAdapter struct {
	app *App
	r   Resource
}

// MakeResourceAdapter returns an implementation of internal/router's
// ResourceController that delegates to the provided Resource implementation.
func MakeResourceAdapter(app *App, r Resource) routerpkg.ResourceController {
	return &resourceAdapter{app: app, r: r}
}

func (a *resourceAdapter) Index(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Index(ctx)
}

func (a *resourceAdapter) New(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.New(ctx)
}

func (a *resourceAdapter) Create(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Create(ctx)
}

func (a *resourceAdapter) Show(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Show(ctx)
}

func (a *resourceAdapter) Edit(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Edit(ctx)
}

func (a *resourceAdapter) Update(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Update(ctx)
}

func (a *resourceAdapter) Destroy(w http.ResponseWriter, req *http.Request) {
	ctx := NewContext(a.app, w, req)
	defer PutContext(ctx)
	a.r.Destroy(ctx)
}

// Example usage (for documentation):
//
//  type UsersController struct{ *flow.Controller }
//
//  func NewUsersController(app *flow.App) *UsersController {
//      return &UsersController{Controller: flow.NewController(app)}
//  }
//
//  func (u *UsersController) Index(ctx *flow.Context) { /* ... */ }
//
//  // wiring with internal router:
//  r := router.New()
//  r.Resources("users", flow.MakeResourceAdapter(app, NewUsersController(app)))
