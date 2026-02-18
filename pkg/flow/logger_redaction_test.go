package flow

import (
	"strings"
	"testing"
)

func TestRedactedValue_SecretKey(t *testing.T) {
	v := RedactedValue("password", "hunter2")
	if s, ok := v.(string); !ok || s != "[REDACTED]" {
		t.Fatalf("expected redacted value, got %#v", v)
	}
}

func TestRedactedValue_LongString(t *testing.T) {
	long := strings.Repeat("A", 80)
	v := RedactedValue("session", long)
	if s, ok := v.(string); !ok || s != "[REDACTED]" {
		t.Fatalf("expected long string to be redacted, got %#v", v)
	}
}

func TestRedactMap(t *testing.T) {
	in := map[string]interface{}{
		"username": "alice",
		"api_key":  "secretkey",
	}
	out := RedactMap(in)
	if out["username"] != "alice" {
		t.Fatalf("username should be unchanged")
	}
	if out["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key should be redacted, got %#v", out["api_key"])
	}
	// ensure original map not mutated
	if in["api_key"] == "[REDACTED]" {
		t.Fatalf("input map mutated")
	}
}

func TestRedactString(t *testing.T) {
	s := "token=abcd1234"
	got := Redact(s, "abcd1234")
	if got != "token=[REDACTED]" {
		t.Fatalf("expected token redacted, got %q", got)
	}
}
