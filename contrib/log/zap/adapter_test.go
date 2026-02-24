package zapadapter

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Basic tests to ensure adapter forwards logs to zap and converts keyvals.
func TestZapAdapter_Logs(t *testing.T) {
	// create an observed core so we can inspect emitted entries
	core, observed := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	defer func() { _ = logger.Sync() }()

	za := &ZapAdapter{L: logger}

	// Use Info with keyvals
	za.Info(context.Background(), "hello", "user", "alice", "id", 123)

	logs := observed.All()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	e := logs[0]
	if e.Message != "hello" {
		t.Fatalf("unexpected message: %s", e.Message)
	}
	// ensure fields include user and id (compare via string representation for robustness)
	foundUser := false
	foundID := false
	for _, f := range e.Context {
		if f.Key == "user" {
			if f.String == "alice" || fmt.Sprint(f.Interface) == "alice" {
				foundUser = true
			}
		}
		if f.Key == "id" {
			if f.Integer == 123 || fmt.Sprint(f.Interface) == "123" {
				foundID = true
			}
		}
	}
	if !foundUser || !foundID {
		t.Fatalf("expected fields user and id in log context; got: %#v", e.Context)
	}
}

func TestNewZapAdapter_NilLogger(t *testing.T) {
	// Should not panic or fail when nil logger provided.
	_ = NewZapAdapter(nil)
}
