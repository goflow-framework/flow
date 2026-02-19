package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// StdLogger is a small adapter that implements the framework Logger interface
// (defined in app.go) using the standard library's log.Logger. It is
// intentionally minimal so projects can provide their own structured
// loggers while adapters remain simple.
type StdLogger struct {
	l *log.Logger
}

// NewStdLogger returns a Logger backed by the provided *log.Logger. If nil is
// provided it will create a default logger writing to stdout.
func NewStdLogger(l *log.Logger) Logger {
	if l == nil {
		l = log.New(os.Stdout, "[flow] ", log.LstdFlags)
	}
	return &StdLogger{l: l}
}

// NewDiscardLogger returns a Logger that discards output. Useful in tests.
func NewDiscardLogger() Logger {
	return &StdLogger{l: log.New(io.Discard, "", 0)}
}

// Printf implements Logger.Printf.
func (s *StdLogger) Printf(format string, v ...interface{}) {
	s.l.Printf(format, v...)
}

// Redaction helpers

var defaultSecretKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"secret":        {},
	"token":         {},
	"access_token":  {},
	"auth":          {},
	"authorization": {},
	"api_key":       {},
}

// IsSecretKey reports whether a key name should be considered secret.
func IsSecretKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	_, ok := defaultSecretKeys[k]
	return ok
}

// RedactedValue returns a redacted representation for the given key and value.
// Rules:
//   - If the key name looks secret (see IsSecretKey) the value is replaced
//     with the literal string "[REDACTED]".
//   - If the value is a string longer than 64 characters it's treated as a token
//     and redacted as well. Other values are returned unchanged.
func RedactedValue(key string, v interface{}) interface{} {
	if IsSecretKey(key) {
		return "[REDACTED]"
	}
	if s, ok := v.(string); ok {
		if len(s) > 64 {
			return "[REDACTED]"
		}
	}
	return v
}

// RedactMap returns a shallow copy of the provided map with secret values
// replaced by the redacted sentinel. It accepts maps with string keys and
// arbitrary values and never mutates the input map.
func RedactMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = RedactedValue(k, v)
	}
	return out
}

// Redact replaces occurrences of any provided secret substrings with the
// marker "[REDACTED]". It is intentionally simple: callers should avoid
// passing very large secret lists. This helper is useful for producing logs
// that omit sensitive values.
func Redact(s string, secrets ...string) string {
	if s == "" || len(secrets) == 0 {
		return s
	}
	out := s
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		out = strings.ReplaceAll(out, sec, "[REDACTED]")
	}
	return out
}

// StructuredLogger is a minimal interface that the framework uses for
// structured logging. Keep it small so adapters are easy to provide.
type StructuredLogger interface {
	Debug(ctx context.Context, msg string, keyvals ...interface{})
	Info(ctx context.Context, msg string, keyvals ...interface{})
	Warn(ctx context.Context, msg string, keyvals ...interface{})
	Error(ctx context.Context, msg string, keyvals ...interface{})
	Log(level string, msg string, fields map[string]interface{})
}

// LoggerAdapter is a small helper that allows providing a nil-safe logger
// to App. Consumers can provide adapters for zap/zerolog in contrib/.
type LoggerAdapter struct {
	L StructuredLogger
}

// Printf implements the legacy Logger interface by forwarding to the
// underlying StructuredLogger as an info-level entry. This makes
// LoggerAdapter usable wherever a Logger is expected.
func (a *LoggerAdapter) Printf(format string, v ...interface{}) {
	if a == nil || a.L == nil {
		return
	}
	a.L.Log("info", fmt.Sprintf(format, v...), nil)
}

// JSONLogger is a tiny JSON-line logger implementing both Printf (so it can
// be used where Logger is expected) and StructuredLogger. It outputs one
// compact JSON object per line with timestamp, level, message and fields.
type JSONLogger struct {
	out io.Writer
}

// NewJSONLogger constructs a JSONLogger writing to out (defaults to stdout).
func NewJSONLogger(out io.Writer) *JSONLogger {
	if out == nil {
		out = os.Stdout
	}
	return &JSONLogger{out: out}
}

// Printf implements the legacy Logger.Printf contract by emitting a JSON
// entry at level=info with the formatted message.
func (j *JSONLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	j.Log("info", msg, nil)
}

// Log implements StructuredLogger. Fields are shallow-redacted before being
// encoded to JSON.
func (j *JSONLogger) Log(level string, msg string, fields map[string]interface{}) {
	entry := map[string]interface{}{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	if len(fields) > 0 {
		entry["fields"] = RedactMap(fields)
	}
	// Best-effort encode and write; failures are ignored to avoid panics in
	// logging paths.
	if b, err := json.Marshal(entry); err == nil {
		j.out.Write(b)
		j.out.Write([]byte("\n"))
	}
}

// keyvalsToMap converts a variadic key/value list into a map for the
// framework's StructuredLogger.Log method. If an odd number of elements is
// provided the last key is ignored.
func keyvalsToMap(kv []interface{}) map[string]interface{} {
	if len(kv) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i < len(kv)-1; i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		m[k] = kv[i+1]
	}
	return m
}

// Convenience helpers to implement StructuredLogger so JSONLogger can be
// used directly where StructuredLogger is preferred.
func (j *JSONLogger) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	j.Log("debug", msg, keyvalsToMap(keyvals))
}
func (j *JSONLogger) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	j.Log("info", msg, keyvalsToMap(keyvals))
}
func (j *JSONLogger) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	j.Log("warn", msg, keyvalsToMap(keyvals))
}
func (j *JSONLogger) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	j.Log("error", msg, keyvalsToMap(keyvals))
}

func (a *LoggerAdapter) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	if a == nil || a.L == nil {
		return
	}
	a.L.Log("debug", msg, keyvalsToMap(keyvals))
}

func (a *LoggerAdapter) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	if a == nil || a.L == nil {
		return
	}
	a.L.Log("info", msg, keyvalsToMap(keyvals))
}

func (a *LoggerAdapter) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	if a == nil || a.L == nil {
		return
	}
	a.L.Log("warn", msg, keyvalsToMap(keyvals))
}

func (a *LoggerAdapter) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	if a == nil || a.L == nil {
		return
	}
	a.L.Log("error", msg, keyvalsToMap(keyvals))
}
