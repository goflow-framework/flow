package flow

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// newTestSlogLogger returns a *SlogLogger that writes JSON lines to buf.
func newTestSlogLogger(buf *bytes.Buffer) *SlogLogger {
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewSlogLogger(slog.New(h))
}

// ── interface compliance ──────────────────────────────────────────────────────

// TestSlogLogger_ImplementsLogger verifies *SlogLogger satisfies Logger.
func TestSlogLogger_ImplementsLogger(t *testing.T) {
	var _ Logger = NewSlogLogger(nil)
}

// TestSlogLogger_ImplementsStructuredLogger verifies *SlogLogger satisfies StructuredLogger.
func TestSlogLogger_ImplementsStructuredLogger(t *testing.T) {
	var _ StructuredLogger = NewSlogLogger(nil)
}

// ── NewSlogLogger ─────────────────────────────────────────────────────────────

// TestNewSlogLogger_NilUsesDefault verifies that passing nil uses slog.Default()
// without panicking.
func TestNewSlogLogger_NilUsesDefault(t *testing.T) {
	sl := NewSlogLogger(nil)
	if sl == nil {
		t.Fatal("expected non-nil SlogLogger")
	}
	if sl.Slog() == nil {
		t.Fatal("expected non-nil underlying *slog.Logger")
	}
}

// TestNewSlogLogger_CustomLogger verifies the provided *slog.Logger is stored.
func TestNewSlogLogger_CustomLogger(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, nil)
	base := slog.New(h)
	sl := NewSlogLogger(base)
	if sl.Slog() != base {
		t.Fatal("Slog() did not return the provided logger")
	}
}

// ── Printf ────────────────────────────────────────────────────────────────────

// TestSlogLogger_Printf_EmitsAtInfo verifies Printf output appears at level INFO.
func TestSlogLogger_Printf_EmitsAtInfo(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Printf("hello %s", "world")
	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("Printf: message not found in output: %s", out)
	}
	if !strings.Contains(out, `"level":"INFO"`) {
		t.Fatalf("Printf: expected INFO level, got: %s", out)
	}
}

// TestSlogLogger_Printf_Formatting verifies fmt-style formatting works.
func TestSlogLogger_Printf_Formatting(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Printf("count=%d ratio=%.2f", 42, 3.14)
	out := buf.String()
	if !strings.Contains(out, "count=42 ratio=3.14") {
		t.Fatalf("Printf format mismatch: %s", out)
	}
}

// ── StructuredLogger methods ──────────────────────────────────────────────────

func TestSlogLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Debug(context.Background(), "debug-msg", "k", "v")
	out := buf.String()
	if !strings.Contains(out, "debug-msg") {
		t.Fatalf("Debug: message not found: %s", out)
	}
	if !strings.Contains(out, `"level":"DEBUG"`) {
		t.Fatalf("Debug: expected DEBUG level: %s", out)
	}
}

func TestSlogLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Info(context.Background(), "info-msg")
	out := buf.String()
	if !strings.Contains(out, "info-msg") || !strings.Contains(out, `"level":"INFO"`) {
		t.Fatalf("Info: unexpected output: %s", out)
	}
}

func TestSlogLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Warn(context.Background(), "warn-msg")
	out := buf.String()
	if !strings.Contains(out, "warn-msg") || !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("Warn: unexpected output: %s", out)
	}
}

func TestSlogLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Error(context.Background(), "error-msg")
	out := buf.String()
	if !strings.Contains(out, "error-msg") || !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("Error: unexpected output: %s", out)
	}
}

// ── Log (level string mapping) ────────────────────────────────────────────────

func TestSlogLogger_Log_LevelMapping(t *testing.T) {
	cases := []struct {
		level    string
		wantSlog string
	}{
		{"debug", `"level":"DEBUG"`},
		{"info", `"level":"INFO"`},
		{"warn", `"level":"WARN"`},
		{"warning", `"level":"WARN"`},
		{"error", `"level":"ERROR"`},
		{"unknown", `"level":"INFO"`}, // fallback
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.level, func(t *testing.T) {
			var buf bytes.Buffer
			sl := newTestSlogLogger(&buf)
			sl.Log(tc.level, "msg", nil)
			out := buf.String()
			if !strings.Contains(out, tc.wantSlog) {
				t.Fatalf("level=%q: expected %q in output: %s", tc.level, tc.wantSlog, out)
			}
		})
	}
}

// TestSlogLogger_Log_FieldsRedacted verifies that secret fields are redacted
// before being forwarded to slog.
func TestSlogLogger_Log_FieldsRedacted(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	sl.Log("info", "user action", map[string]interface{}{
		"password": "super-secret",
		"user":     "alice",
	})
	out := buf.String()
	if strings.Contains(out, "super-secret") {
		t.Fatalf("secret password should be redacted, got: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker in output: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Fatalf("non-secret field 'user' should be present: %s", out)
	}
}

// ── WithLogger integration ────────────────────────────────────────────────────

// TestSlogLogger_WithLogger_Integration verifies that an App wired with
// WithLogger(NewSlogLogger(...)) routes its internal Printf calls through slog.
func TestSlogLogger_WithLogger_Integration(t *testing.T) {
	var buf bytes.Buffer
	sl := newTestSlogLogger(&buf)
	a := New("slog-app", WithLogger(sl))
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.Shutdown(ctx)
	// Shutdown logs "shutting down <name>" — verify it went through slog.
	out := buf.String()
	if !strings.Contains(out, "slog-app") {
		t.Fatalf("expected app name in slog output during shutdown, got: %s", out)
	}
}
