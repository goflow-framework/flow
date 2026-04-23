package flow_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/goflow-framework/flow/pkg/flow"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeCtxWithApp(app *flow.App, r *http.Request) *flow.Context {
	w := httptest.NewRecorder()
	return flow.NewContext(app, w, r)
}

func getReq() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

// ---------------------------------------------------------------------------
// App.Validator / App.SetValidator
// ---------------------------------------------------------------------------

func TestAppValidator_NilByDefault(t *testing.T) {
	t.Parallel()
	app := flow.New("test")
	if app.Validator() != nil {
		t.Fatal("expected nil validator by default, got non-nil")
	}
}

func TestAppSetValidator_StoresAndReturns(t *testing.T) {
	t.Parallel()
	app := flow.New("test")
	v := validator.New()
	app.SetValidator(v)
	if app.Validator() != v {
		t.Fatal("Validator() did not return the value set via SetValidator")
	}
}

func TestAppSetValidator_NilIsIgnored(t *testing.T) {
	t.Parallel()
	app := flow.New("test")
	v := validator.New()
	app.SetValidator(v)
	app.SetValidator(nil) // must not replace with nil
	if app.Validator() != v {
		t.Fatal("SetValidator(nil) should not overwrite an existing validator")
	}
}

// ---------------------------------------------------------------------------
// WithValidator option
// ---------------------------------------------------------------------------

func TestWithValidator_WiresOnApp(t *testing.T) {
	t.Parallel()
	v := validator.New()
	app := flow.New("test", flow.WithValidator(v))
	if app.Validator() != v {
		t.Fatal("WithValidator did not wire the validator onto the App")
	}
}

func TestWithValidator_NilIsNoop(t *testing.T) {
	t.Parallel()
	app := flow.New("test", flow.WithValidator(nil))
	if app.Validator() != nil {
		t.Fatal("WithValidator(nil) should leave Validator() nil")
	}
}

// ---------------------------------------------------------------------------
// Context.Validate — uses App validator when present
// ---------------------------------------------------------------------------

type validateInput struct {
	Email string `validate:"required,email"`
}

func TestContextValidate_UsesAppValidator(t *testing.T) {
	t.Parallel()
	v := validator.New()
	app := flow.New("test", flow.WithValidator(v))
	ctx := makeCtxWithApp(app, getReq())

	good := validateInput{Email: "user@example.com"}
	if err := ctx.Validate(&good); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	bad := validateInput{Email: "not-an-email"}
	if err := ctx.Validate(&bad); err == nil {
		t.Fatal("expected validation error for bad email, got nil")
	}
}

func TestContextValidate_FallsBackToPkgValidator_WhenNoApp(t *testing.T) {
	t.Parallel()
	// nil App → falls back to package-level validator
	ctx := makeCtxWithApp(nil, getReq())

	good := validateInput{Email: "user@example.com"}
	if err := ctx.Validate(&good); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	bad := validateInput{}
	if err := ctx.Validate(&bad); err == nil {
		t.Fatal("expected validation error for missing required field, got nil")
	}
}

func TestContextValidate_FallsBackToPkgValidator_WhenAppHasNone(t *testing.T) {
	t.Parallel()
	// App with no validator set → falls back to package-level validator
	app := flow.New("test") // no WithValidator
	ctx := makeCtxWithApp(app, getReq())

	bad := validateInput{Email: "bad"}
	if err := ctx.Validate(&bad); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Each App has an isolated validator (no cross-App sharing)
// ---------------------------------------------------------------------------

func TestValidator_IsolatedPerApp(t *testing.T) {
	t.Parallel()

	// v1 has a custom tag that always fails; v2 does not.
	v1 := validator.New()
	if err := v1.RegisterValidation("alwaysfail", func(fl validator.FieldLevel) bool {
		return false
	}); err != nil {
		t.Fatalf("RegisterValidation: %v", err)
	}

	v2 := validator.New()

	app1 := flow.New("app1", flow.WithValidator(v1))
	app2 := flow.New("app2", flow.WithValidator(v2))

	type Input struct {
		Name string `validate:"alwaysfail"`
	}

	ctx1 := makeCtxWithApp(app1, getReq())
	if err := ctx1.Validate(&Input{Name: "anything"}); err == nil {
		t.Fatal("app1: expected alwaysfail validation to reject, got nil")
	}

	ctx2 := makeCtxWithApp(app2, getReq())
	// v2 does not know about "alwaysfail" tag → should error with
	// "undefined validation function" not a validation failure
	// (either way, the two validators are demonstrably independent)
	if app2.Validator() == app1.Validator() {
		t.Fatal("app1 and app2 share the same validator instance — isolation broken")
	}

	_ = ctx2 // suppress unused warning
}

// ---------------------------------------------------------------------------
// Concurrent safety — race detector will catch violations
// ---------------------------------------------------------------------------

func TestSetValidator_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	app := flow.New("race-test")

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			app.SetValidator(validator.New())
		}()
	}

	// Concurrent readers (via Context.Validate)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ctx := makeCtxWithApp(app, getReq())
			_ = ctx.Validate(&validateInput{Email: "user@example.com"})
		}()
	}

	wg.Wait()
}

func TestSetValidatorPkg_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent writers via package-level SetValidator
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			flow.SetValidator(validator.New())
		}()
	}

	// Concurrent readers via a Context with no App
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ctx := makeCtxWithApp(nil, getReq())
			_ = ctx.Validate(&validateInput{Email: "user@example.com"})
		}()
	}

	wg.Wait()
}
