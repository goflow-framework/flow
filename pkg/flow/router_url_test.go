package flow

import (
    "testing"
)

func TestRouterURLAndGetNamed(t *testing.T) {
    app := New("test-app")
    r := NewRouter(app)

    // Register a named route with a path parameter
    r.GetNamed("greet", "/greet/:name", func(ctx *Context) {
        // handler body not needed for URL generation test
        _ = ctx
    })

    // Verify URL generation with param
    u, err := r.URL("greet", map[string]string{"name": "alice"})
    if err != nil {
        t.Fatalf("unexpected error generating URL: %v", err)
    }
    if u != "/greet/alice" {
        t.Fatalf("expected /greet/alice, got %s", u)
    }

    // Missing param should return an error
    if _, err := r.URL("greet", map[string]string{}); err == nil {
        t.Fatalf("expected error when missing param, got nil")
    }

    // Register RESTful resources and ensure named route for show exists
    users := NewUsersController(app)
    if err := r.Resources("users", users); err != nil {
        t.Fatalf("Resources error: %v", err)
    }

    u2, err := r.URL("users_show", map[string]string{"id": "42"})
    if err != nil {
        t.Fatalf("unexpected error generating users_show URL: %v", err)
    }
    if u2 != "/users/42" {
        t.Fatalf("expected /users/42, got %s", u2)
    }

    // Unknown route name returns error
    if _, err := r.URL("no_such_route", nil); err == nil {
        t.Fatalf("expected error for unknown route name, got nil")
    }
}
