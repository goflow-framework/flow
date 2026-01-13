// Package flow exposes the public API surface of the Flow framework.
//
// Context is the request-scoped helper passed to controller actions. It
// wraps http.ResponseWriter and *http.Request and provides convenience
// helpers for common tasks: rendering JSON, rendering templates, reading
// parameters, redirects, and binding request bodies.
//
// Design notes:
//   - Context is deliberately small and explicit. It does not perform magic.
//   - Parameter access reads from the request context (the router injects
//     parameters). This keeps Context decoupled from routing internals while
//     still permitting efficient access via the internal router helper.
//   - Rendering helpers return errors so controller code can decide how to
//     handle failures (log, render an error page, etc.).
//
// TODO: add helper for rendering layouts, template caching, and streaming
// responses when those features are required.
package flow

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sync"

	routerpkg "github.com/dministrator/flow/internal/router"
)

// Context is a small, testable wrapper around ResponseWriter and Request.
// Controllers should accept or construct a Context rather than using global
// state.
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
	if UseContextPool {
		contextPool.Put(c)
	}
}

// Params returns the path parameters extracted by the router for this request.
// It always returns a non-nil map.
func (c *Context) Params() map[string]string {
	return routerpkg.ParamsFromContext(c.R.Context())
}

// Param returns the named path parameter or an empty string if missing.
func (c *Context) Param(name string) string {
	return routerpkg.Param(c.R, name)
}

// SetHeader sets a header on the response.
func (c *Context) SetHeader(key, value string) {
	c.W.Header().Set(key, value)
}

// Status sets the HTTP status code for the response. It immediately writes
// the header so subsequent writes will use the status. Calling Status more
// than once is allowed; the first call wins from the net/http perspective.
func (c *Context) Status(code int) {
	c.status = code
	c.W.WriteHeader(code)
}

// JSON writes v as a JSON response with the provided status code.
// It sets Content-Type to application/json; charset=utf-8.
func (c *Context) JSON(status int, v interface{}) error {
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	if status == 0 {
		status = http.StatusOK
	}
	c.Status(status)
	enc := json.NewEncoder(c.W)
	// Use compact encoding by default. Caller can pre-encode for custom
	// options.
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("render json: %w", err)
	}
	return nil
}

// RenderTemplate executes the provided template. The caller must supply a
// parsed *template.Template (template caching is outside Context's
// responsibility) and the name of the template to execute.
func (c *Context) RenderTemplate(t *template.Template, name string, data interface{}) error {
	if t == nil {
		return fmt.Errorf("render template: template is nil")
	}
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	// default to 200 OK if not previously set
	if c.status == 0 {
		c.Status(http.StatusOK)
	}
	if err := t.ExecuteTemplate(c.W, name, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}
	return nil
}

// Redirect sends an HTTP redirect to the client.
func (c *Context) Redirect(urlStr string, code int) {
	if code == 0 {
		code = http.StatusFound
	}
	http.Redirect(c.W, c.R, urlStr, code)
}

// BindJSON decodes the request body into dst. dst must be a pointer. This
// helper ensures the request body is closed and returns descriptive errors.
func (c *Context) BindJSON(dst interface{}) error {
	if dst == nil {
		return fmt.Errorf("bind json: dst is nil")
	}
	defer func() {
		// best-effort close of body for servers that don't rely on it
		io.Copy(io.Discard, c.R.Body)
		c.R.Body.Close()
	}()
	dec := json.NewDecoder(c.R.Body)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("bind json: %w", err)
	}
	return nil
}

// FormValue is a small helper to retrieve form values (POST/PUT). It calls
// ParseForm if necessary.
func (c *Context) FormValue(key string) string {
	// ParseForm is idempotent and safe to call multiple times.
	_ = c.R.ParseForm()
	return c.R.FormValue(key)
}

// Render is a convenience helper that uses the App's ViewManager to render
// the named template. It returns an error if views are not configured.
func (c *Context) Render(name string, data interface{}) error {
	if c.App == nil || c.App.Views == nil {
		return fmt.Errorf("render: views not configured")
	}
	return c.App.Views.Render(name, data, c)
}

// Session returns the session store for the current request, or nil if
// sessions are not configured. Use Session().Get/Set/Delete to manage
// session data. Session writes a cookie on Set/Delete/Save.
func (c *Context) Session() *Session {
	return FromContext(c.R.Context())
}

// Flash helpers — store simple flash messages in session under the "_flash"
// key. Each flash is a map[string]string with keys "kind" and "msg".
type FlashEntry struct {
	Kind string
	Msg  string
}

// AddFlash adds a flash message of a given kind to the session.
func (c *Context) AddFlash(kind, msg string) error {
	s := c.Session()
	if s == nil {
		return fmt.Errorf("flash: session not configured")
	}
	var list []map[string]string
	if v, ok := s.Get("_flash"); ok {
		if arr, ok := v.([]interface{}); ok {
			for _, it := range arr {
				if m, ok := it.(map[string]interface{}); ok {
					entry := map[string]string{}
					if k, ok := m["kind"].(string); ok {
						entry["kind"] = k
					}
					if mmsg, ok := m["msg"].(string); ok {
						entry["msg"] = mmsg
					}
					list = append(list, entry)
				}
			}
		}
	}
	list = append(list, map[string]string{"kind": kind, "msg": msg})
	return s.Set("_flash", list)
}

// Flashes returns and clears flash messages from the session.
func (c *Context) Flashes() ([]FlashEntry, error) {
	s := c.Session()
	if s == nil {
		return nil, fmt.Errorf("flash: session not configured")
	}
	v, _ := s.Get("_flash")
	var entries []FlashEntry
	if v != nil {
		if arr, ok := v.([]interface{}); ok {
			for _, it := range arr {
				if m, ok := it.(map[string]interface{}); ok {
					fe := FlashEntry{}
					if k, ok := m["kind"].(string); ok {
						fe.Kind = k
					}
					if mm, ok := m["msg"].(string); ok {
						fe.Msg = mm
					}
					entries = append(entries, fe)
				}
			}
		}
	}
	// clear flashes
	_ = s.Delete("_flash")
	return entries, nil
}

// Error writes a simple error response with the provided status and message.
// It is intentionally minimal; projects may replace this with HTML error
// pages in their App configuration.
func (c *Context) Error(status int, msg string) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	c.Status(status)
	_, _ = c.W.Write([]byte(msg))
}

// TODO: add helpers for file uploads, streaming responses, template caching,
// secure cookie helpers, and content negotiation as the framework evolves.
