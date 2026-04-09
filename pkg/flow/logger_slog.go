package flow

import (
	"context"
	"fmt"
	"log/slog"
)

// SlogLogger is an adapter that satisfies the framework's Logger interface
// (Printf) and StructuredLogger interface using a *slog.Logger as the
// backend. It bridges the older Printf-based contract with the structured
// log/slog package introduced in Go 1.21.
//
// Usage:
//
// app := flow.New("myapp",
//
//	flow.WithLogger(flow.NewSlogLogger(slog.Default())),
//
// )
//
// Log levels:
//   - Printf  → slog.Info   (matches existing StdLogger behaviour)
//   - Info    → slog.Info
//   - Debug   → slog.Debug
//   - Warn    → slog.Warn
//   - Error   → slog.Error
type SlogLogger struct {
	l *slog.Logger
}

// NewSlogLogger returns a Logger (and StructuredLogger) backed by the given
// *slog.Logger. If l is nil, slog.Default() is used so callers can write:
//
// flow.WithLogger(flow.NewSlogLogger(nil))
func NewSlogLogger(l *slog.Logger) *SlogLogger {
	if l == nil {
		l = slog.Default()
	}
	return &SlogLogger{l: l}
}

// Printf implements Logger. The format string is expanded with fmt.Sprintf
// and emitted at slog.LevelInfo, matching the behaviour of StdLogger.Printf.
func (s *SlogLogger) Printf(format string, v ...interface{}) {
	s.l.Info(fmt.Sprintf(format, v...))
}

// Debug implements StructuredLogger.
func (s *SlogLogger) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	s.l.DebugContext(ctx, msg, keyvals...)
}

// Info implements StructuredLogger.
func (s *SlogLogger) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	s.l.InfoContext(ctx, msg, keyvals...)
}

// Warn implements StructuredLogger.
func (s *SlogLogger) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	s.l.WarnContext(ctx, msg, keyvals...)
}

// Error implements StructuredLogger.
func (s *SlogLogger) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	s.l.ErrorContext(ctx, msg, keyvals...)
}

// Log implements StructuredLogger. The level string is mapped to the
// corresponding slog.Level; unknown values fall back to slog.LevelInfo.
// Fields are passed as slog key-value pairs after redaction.
func (s *SlogLogger) Log(level string, msg string, fields map[string]interface{}) {
	sl := slogLevel(level)
	if !s.l.Enabled(context.Background(), sl) {
		return
	}
	args := fieldsToSlogArgs(RedactMap(fields))
	s.l.Log(context.Background(), sl, msg, args...)
}

// Slog returns the underlying *slog.Logger so callers can use slog-specific
// features (e.g. WithGroup, WithAttrs) after obtaining an SlogLogger.
func (s *SlogLogger) Slog() *slog.Logger {
	return s.l
}

// slogLevel maps a StructuredLogger level string to a slog.Level.
func slogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fieldsToSlogArgs converts a map[string]interface{} into a flat slice of
// alternating key/value arguments compatible with slog's variadic API.
func fieldsToSlogArgs(fields map[string]interface{}) []interface{} {
	if len(fields) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return args
}
