package flow

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONLogger_NonSecretPreserved(t *testing.T) {
	buf := &bytes.Buffer{}
	jl := NewJSONLogger(buf)
	jl.Log("info", "hello", map[string]interface{}{"username": "alice"})

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}
	fieldsI, ok := entry["fields"]
	if !ok {
		t.Fatalf("expected fields in log entry")
	}
	fields := fieldsI.(map[string]interface{})
	if got, ok := fields["username"]; !ok {
		t.Fatalf("expected username field present")
	} else if got != "alice" {
		t.Fatalf("expected username preserved, got %v", got)
	}
}

func TestRedactMapWithConfig_LongString(t *testing.T) {
	long := "abcdefgh"
	cfg := RedactionConfig{Keys: map[string]struct{}{}, MaxLen: 5}
	out := RedactMapWithConfig(&cfg, map[string]interface{}{"session": long})
	if out["session"] != "[REDACTED]" {
		t.Fatalf("expected long string to be redacted, got %#v", out["session"])
	}
}
