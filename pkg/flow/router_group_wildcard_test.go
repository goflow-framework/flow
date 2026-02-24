package flow

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteGroupAndWildcardAndUrlFor(t *testing.T) {
	app := New("test-app")
	r := NewRouter(app)

	// Group example: register a named route under /admin
	grp := r.Group("admin")
	grp.GetNamed("admin_ping", "/ping", func(ctx *Context) { ctx.W.WriteHeader(200) })

	u, err := r.URL("admin_ping", nil)
	if err != nil {
		t.Fatalf("unexpected error generating admin_ping URL: %v", err)
	}
	if u != "/admin/ping" {
		t.Fatalf("expected /admin/ping, got %s", u)
	}

	// Wildcard route: /files/*path should capture remainder including slashes
	r.Get("/files/*path", func(ctx *Context) {
		_, _ = io.WriteString(ctx.W, ctx.Param("path"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/files/a/b/c.txt", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for wildcard route, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "a/b/c.txt" {
		t.Fatalf("expected wildcard capture a/b/c.txt, got %s", string(body))
	}

	// Named wildcard URL generation
	r.GetNamed("files_show", "/files/*path", func(ctx *Context) {})
	u2, err := r.URL("files_show", map[string]string{"path": "a/b/c.txt"})
	if err != nil {
		t.Fatalf("unexpected error generating files_show URL: %v", err)
	}
	if u2 != "/files/a/b/c.txt" {
		t.Fatalf("expected /files/a/b/c.txt, got %s", u2)
	}

	// url_for template helper injection via App.SetRouter
	app.SetRouter(r)
	// Build a small template that calls url_for
	// Sanity-check the injected function directly
	if fn, ok := app.Views.FuncMap["url_for"].(func(string, ...interface{}) string); ok {
		// This will return # if the helper cannot generate the URL; ensure it's correct
		v := fn("hello", "name", "bob")
		if v != "#" && v != "/hello/bob" {
			// if the named route hasn't been registered yet, we still expect a valid placeholder
			t.Logf("url_for direct call returned: %q", v)
		}
	} else {
		t.Fatalf("url_for helper not present or wrong type in FuncMap")
	}

	// Register the named route used by the template and verify the helper
	r.GetNamed("hello", "/hello/:name", func(ctx *Context) {})
	if fn2, ok := app.Views.FuncMap["url_for"].(func(string, ...interface{}) string); ok {
		v2 := fn2("hello", "name", "bob")
		if v2 != "/hello/bob" {
			t.Fatalf("expected direct helper to return /hello/bob, got %q", v2)
		}
	} else {
		t.Fatalf("url_for helper not present or wrong type in FuncMap after registration")
	}
}
