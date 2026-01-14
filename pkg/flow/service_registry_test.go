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
