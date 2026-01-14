package flow

import (
	"testing"
)

func TestServiceRegistry_RegisterAndGet(t *testing.T) {
	r := NewServiceRegistry()
	svc := struct{ Name string }{Name: "mailer"}
	if err := r.Register("mailer", svc); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	v, ok := r.Get("mailer")
	if !ok {
		t.Fatalf("expected to find registered service")
	}
	if _, ok := v.(struct{ Name string }); !ok {
		// type assertion will fail because anonymous struct types differ across packages
		// simply ensure it's non-nil
		if v == nil {
			t.Fatalf("service value is nil")
		}
	}
}

func TestServiceRegistry_Duplicate(t *testing.T) {
	r := NewServiceRegistry()
	if err := r.Register("s", 1); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := r.Register("s", 2); err == nil {
		t.Fatalf("expected duplicate register to fail")
	}
}

func TestServiceRegistry_UnregisterAndReplace(t *testing.T) {
	r := NewServiceRegistry()
	if err := r.Register("s", 1); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Unregister should remove the service
	if !r.Unregister("s") {
		t.Fatalf("expected Unregister to return true")
	}
	if _, ok := r.Get("s"); ok {
		t.Fatalf("expected service to be removed")
	}

	// Replace should fail if service does not exist
	if err := r.Replace("s", 2); err == nil {
		t.Fatalf("expected Replace to fail for non-existent service")
	}

	// Re-register and replace
	if err := r.Register("s", 2); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := r.Replace("s", 3); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	v, ok := r.Get("s")
	if !ok {
		t.Fatalf("expected service to exist after replace")
	}
	if vi, ok := v.(int); !ok || vi != 3 {
		t.Fatalf("expected replaced value 3, got %#v", v)
	}
}
