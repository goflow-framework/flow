package flow

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddleware_EmitsStructuredJSON(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)

	mw := LoggingMiddlewareWithRedaction(jl)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test-path", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected log output, got empty")
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 log line, got %d", len(lines))
	}

	// parse first line and assert structured fields exist
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to unmarshal log JSON: %v; raw=%s", err, lines[0])
	}
	if entry["level"] != "info" {
		t.Fatalf("expected level=info, got %v", entry["level"])
	}
	fields, ok := entry["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected fields map in log entry, got: %#v", entry["fields"])
	}
	if fields["method"] != "GET" {
		t.Fatalf("expected method=GET, got %#v", fields["method"])
	}
	if fields["path"] != "/test-path" {
		t.Fatalf("expected path=/test-path, got %#v", fields["path"])
	}
}

func TestJSONLogger_RedactsFields(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)

	fields := map[string]interface{}{
		"username": "alice",
		"api_key":  "secret-key-123",
		"token":    strings.Repeat("X", 100),
	}
	jl.Log("warn", "sensitive test", fields)

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected json output from JSONLogger")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatalf("failed to unmarshal json logger output: %v", err)
	}
	f, ok := entry["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected fields map, got %#v", entry["fields"])
	}
	if f["username"] != "alice" {
		t.Fatalf("username should be unchanged, got %#v", f["username"])
	}
	if f["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key should be redacted, got %#v", f["api_key"])
	}
	if f["token"] != "[REDACTED]" {
		t.Fatalf("long token should be redacted, got %#v", f["token"])
	}
}
