package flow

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// UsersController is a minimal Resource implementation used for tests.
type UsersController struct{ *Controller }

func NewUsersController(app *App) *UsersController {
	return &UsersController{Controller: NewController(app)}
}

func (u *UsersController) Index(ctx *Context)  { ctx.ResponseWriter().WriteHeader(200) }
func (u *UsersController) New(ctx *Context)    { ctx.ResponseWriter().WriteHeader(200) }
func (u *UsersController) Create(ctx *Context) { ctx.ResponseWriter().WriteHeader(200) }
func (u *UsersController) Show(ctx *Context) {
	// echo the :id param
	_, _ = ctx.ResponseWriter().Write([]byte(ctx.Param("id")))
}
func (u *UsersController) Edit(ctx *Context)    { ctx.ResponseWriter().WriteHeader(200) }
func (u *UsersController) Update(ctx *Context)  { ctx.ResponseWriter().WriteHeader(200) }
func (u *UsersController) Destroy(ctx *Context) { ctx.ResponseWriter().WriteHeader(200) }

func TestPublicRouterIntegration(t *testing.T) {
	app := New("test-app")
	r := NewRouter(app)

	// simple GET with Context handler
	r.Get("/hello", func(ctx *Context) {
		_, _ = ctx.ResponseWriter().Write([]byte("world"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /hello, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "world" {
		t.Fatalf("unexpected body: %s", string(body))
	}

	// resources registration and param extraction
	users := NewUsersController(app)
	if err := r.Resources("users", users); err != nil {
		t.Fatalf("Resources error: %v", err)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/42", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /users/42, got %d", rr.Code)
	}
	body, _ = io.ReadAll(rr.Body)
	if string(body) != "42" {
		t.Fatalf("expected body 42, got %s", string(body))
	}

	// method not allowed: POST to a GET-only route
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/hello", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST /hello, got %d", rr.Code)
	}
}
