package logging

import (
	"context"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapAdapterImplementsStructuredLoggerAndHelpers(t *testing.T) {
	// Create an observer core to capture logs.
	core, observed := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	defer logger.Sync()

	// Build adapter and assert non-nil and interface compliance.
	sl := NewZapAdapter(logger)
	if sl == nil {
		t.Fatal("NewZapAdapter returned nil")
	}
	var _ flow.StructuredLogger = sl

	// Use the helper method which should emit a debug-level entry.
	sl.Debug(context.Background(), "test-debug", "k1", "v1")

	entries := observed.TakeAll()
	if len(entries) == 0 {
		t.Fatalf("expected at least 1 log entry, got 0")
	}
	// Find an entry with our message.
	found := false
	for _, e := range entries {
		if e.Message == "test-debug" && e.Level == zap.DebugLevel {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find expected debug entry; entries: %#v", entries)
	}
}
