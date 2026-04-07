package flow_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
)

// makeCtx is a helper that builds a *flow.Context from a raw *http.Request.
func makeCtx(r *http.Request) *flow.Context {
	w := httptest.NewRecorder()
	return flow.NewContext(nil, w, r)
}

// ---------------------------------------------------------------------------
// BindForm
// ---------------------------------------------------------------------------

func TestBindForm_BasicTypes(t *testing.T) {
	type Form struct {
		Name  string  `form:"name"`
		Age   int     `form:"age"`
		Score float64 `form:"score"`
		Admin bool    `form:"admin"`
	}

	body := url.Values{
		"name":  {"Alice"},
		"age":   {"30"},
		"score": {"9.5"},
		"admin": {"true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := makeCtx(req)
	var f Form
	if err := ctx.BindForm(&f); err != nil {
		t.Fatalf("BindForm error: %v", err)
	}
	if f.Name != "Alice" {
		t.Errorf("Name: got %q, want %q", f.Name, "Alice")
	}
	if f.Age != 30 {
		t.Errorf("Age: got %d, want 30", f.Age)
	}
	if f.Score != 9.5 {
		t.Errorf("Score: got %f, want 9.5", f.Score)
	}
	if !f.Admin {
		t.Errorf("Admin: got false, want true")
	}
}

func TestBindForm_DefaultTagLowercase(t *testing.T) {
	// No form tag — should fall back to lowercase field name.
	type Form struct {
		Username string
	}
	body := url.Values{"username": {"bob"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := makeCtx(req)
	var f Form
	if err := ctx.BindForm(&f); err != nil {
		t.Fatalf("BindForm error: %v", err)
	}
	if f.Username != "bob" {
		t.Errorf("Username: got %q, want %q", f.Username, "bob")
	}
}

func TestBindForm_NilDst(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := makeCtx(req)
	if err := ctx.BindForm(nil); err == nil {
		t.Fatal("expected error for nil dst, got nil")
	}
}

func TestBindForm_SliceField(t *testing.T) {
	type Form struct {
		Tags []string `form:"tags"`
	}
	body := url.Values{"tags": {"go", "web", "flow"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := makeCtx(req)
	var f Form
	if err := ctx.BindForm(&f); err != nil {
		t.Fatalf("BindForm error: %v", err)
	}
	if len(f.Tags) != 3 {
		t.Errorf("Tags: got %v, want 3 items", f.Tags)
	}
}

// ---------------------------------------------------------------------------
// BindQuery
// ---------------------------------------------------------------------------

func TestBindQuery_BasicTypes(t *testing.T) {
	type Query struct {
		Q    string `form:"q"`
		Page int    `form:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?q=hello&page=3", nil)
	ctx := makeCtx(req)

	var q Query
	if err := ctx.BindQuery(&q); err != nil {
		t.Fatalf("BindQuery error: %v", err)
	}
	if q.Q != "hello" {
		t.Errorf("Q: got %q, want %q", q.Q, "hello")
	}
	if q.Page != 3 {
		t.Errorf("Page: got %d, want 3", q.Page)
	}
}

func TestBindQuery_MissingKey(t *testing.T) {
	type Query struct {
		Q    string `form:"q"`
		Page int    `form:"page"`
	}

	req := httptest.NewRequest(http.MethodGet, "/?q=hello", nil)
	ctx := makeCtx(req)

	var q Query
	if err := ctx.BindQuery(&q); err != nil {
		t.Fatalf("BindQuery error: %v", err)
	}
	// Page should remain zero-value since it was not in the query.
	if q.Page != 0 {
		t.Errorf("Page: got %d, want 0", q.Page)
	}
}

func TestBindQuery_NilDst(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)
	if err := ctx.BindQuery(nil); err == nil {
		t.Fatal("expected error for nil dst, got nil")
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate_RequiredFieldPresent(t *testing.T) {
	type Input struct {
		Email string `validate:"required,email"`
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)

	in := Input{Email: "user@example.com"}
	if err := ctx.Validate(&in); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}

func TestValidate_RequiredFieldMissing(t *testing.T) {
	type Input struct {
		Email string `validate:"required,email"`
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)

	in := Input{} // Email is empty → required fails
	if err := ctx.Validate(&in); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestValidate_InvalidEmail(t *testing.T) {
	type Input struct {
		Email string `validate:"required,email"`
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)

	in := Input{Email: "not-an-email"}
	if err := ctx.Validate(&in); err == nil {
		t.Fatal("expected validation error for bad email, got nil")
	}
}

func TestValidate_MinConstraint(t *testing.T) {
	type Input struct {
		Password string `validate:"required,min=8"`
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)

	in := Input{Password: "short"}
	if err := ctx.Validate(&in); err == nil {
		t.Fatal("expected validation error for short password, got nil")
	}
}

func TestValidate_NilDst(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := makeCtx(req)
	if err := ctx.Validate(nil); err == nil {
		t.Fatal("expected error for nil dst, got nil")
	}
}

// ---------------------------------------------------------------------------
// BindForm + Validate together (the common usage pattern)
// ---------------------------------------------------------------------------

func TestBindFormAndValidate_RoundTrip(t *testing.T) {
	type SignupForm struct {
		Email    string `form:"email"    validate:"required,email"`
		Password string `form:"password" validate:"required,min=8"`
	}

	body := url.Values{
		"email":    {"alice@example.com"},
		"password": {"supersecret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := makeCtx(req)

	var f SignupForm
	if err := ctx.BindForm(&f); err != nil {
		t.Fatalf("BindForm error: %v", err)
	}
	if err := ctx.Validate(&f); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}

func TestBindFormAndValidate_ValidationFails(t *testing.T) {
	type SignupForm struct {
		Email    string `form:"email"    validate:"required,email"`
		Password string `form:"password" validate:"required,min=8"`
	}

	body := url.Values{
		"email":    {"not-an-email"},
		"password": {"hi"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := makeCtx(req)

	var f SignupForm
	if err := ctx.BindForm(&f); err != nil {
		t.Fatalf("BindForm error: %v", err)
	}
	if err := ctx.Validate(&f); err == nil {
		t.Fatal("expected validation errors, got nil")
	}
}
