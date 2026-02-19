package logging

import (
    "bytes"
    "context"
    "strings"
    "testing"

    "github.com/rs/zerolog"
    "github.com/undiegomejia/flow/pkg/flow"
)

func TestZerologAdapterImplementsStructuredLoggerAndHelpers(t *testing.T) {
    var buf bytes.Buffer
    z := zerolog.New(&buf).With().Timestamp().Logger()

    sl := NewZerologAdapter(&z)
    if sl == nil {
        t.Fatal("NewZerologAdapter returned nil")
    }
    var _ flow.StructuredLogger = sl

    sl.Debug(context.Background(), "ztest", "k1", "v1")

    out := buf.String()
    if !strings.Contains(out, "ztest") {
        t.Fatalf("expected output to contain message 'ztest', got: %q", out)
    }
    if !strings.Contains(out, "k1") || !strings.Contains(out, "v1") {
        t.Fatalf("expected output to contain fields k1/v1, got: %q", out)
    }
}
