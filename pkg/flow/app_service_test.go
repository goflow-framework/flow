package flow

import (
	"errors"
	"testing"
)

func TestApp_RegisterAndGetService(t *testing.T) {
	app := New("svc-test")
	if err := app.RegisterService("mailer", "smtp://localhost"); err != nil {
		t.Fatalf("RegisterService failed: %v", err)
	}
	v, ok := app.GetService("mailer")
	if !ok {
		t.Fatalf("expected service to be present")
	}
	if s, _ := v.(string); s == "" {
		t.Fatalf("unexpected service value: %v", v)
	}
}

// ---------------------------------------------------------------------------
// Generic helpers: RegisterServiceTyped / GetServiceTyped
// ---------------------------------------------------------------------------

type testMailer struct{ addr string }
type testCache struct{}

func TestGetServiceTyped_HappyPath(t *testing.T) {
	app := New("typed-svc-test")
	m := &testMailer{addr: "smtp://localhost"}
	if err := RegisterServiceTyped(app, "mailer", m); err != nil {
		t.Fatalf("RegisterServiceTyped: %v", err)
	}

	got, ok := GetServiceTyped[*testMailer](app, "mailer")
	if !ok {
		t.Fatal("GetServiceTyped: expected ok=true, got false")
	}
	if got != m {
		t.Fatalf("GetServiceTyped: expected same pointer, got %v", got)
	}
}

func TestGetServiceTyped_NotFound(t *testing.T) {
	app := New("typed-svc-notfound")
	_, ok := GetServiceTyped[*testMailer](app, "missing")
	if ok {
		t.Fatal("GetServiceTyped: expected ok=false for missing service, got true")
	}
}

func TestGetServiceTyped_WrongType(t *testing.T) {
	app := New("typed-svc-wrongtype")
	// register a *testMailer but try to retrieve as *testCache
	if err := RegisterServiceTyped(app, "svc", &testMailer{addr: "x"}); err != nil {
		t.Fatalf("RegisterServiceTyped: %v", err)
	}
	_, ok := GetServiceTyped[*testCache](app, "svc")
	if ok {
		t.Fatal("GetServiceTyped: expected ok=false for wrong type, got true")
	}
}

func TestRegisterServiceTyped_DuplicateReturnsError(t *testing.T) {
	app := New("typed-dup-test")
	if err := RegisterServiceTyped(app, "svc", &testMailer{}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := RegisterServiceTyped(app, "svc", &testMailer{})
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
}

func TestGetServiceTyped_NilApp(t *testing.T) {
	var app *App
	_, ok := GetServiceTyped[*testMailer](app, "svc")
	if ok {
		t.Fatal("expected ok=false for nil app, got true")
	}
}

func TestRegisterServiceTyped_NilApp(t *testing.T) {
	var app *App
	err := RegisterServiceTyped(app, "svc", &testMailer{})
	if err == nil {
		t.Fatal("expected error for nil app, got nil")
	}
}

func TestGetServiceTyped_Interface(t *testing.T) {
	// Register a concrete type as its interface — common real-world pattern.
	app := New("typed-iface-test")
	svc := errors.New("sentinel")
	if err := RegisterServiceTyped[error](app, "err-svc", svc); err != nil {
		t.Fatalf("RegisterServiceTyped: %v", err)
	}
	got, ok := GetServiceTyped[error](app, "err-svc")
	if !ok {
		t.Fatal("GetServiceTyped[error]: expected ok=true")
	}
	if got != svc {
		t.Fatalf("GetServiceTyped[error]: wrong value %v", got)
	}
}
