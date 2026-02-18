package flow

import (
    "io"
    "log"
    "os"
    "strings"
)

// StdLogger is a small adapter that implements the framework Logger interface
// (defined in app.go) using the standard library's log.Logger. It is
// intentionally minimal so projects can provide their own structured
// loggers while adapters remain simple.
type StdLogger struct{
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
// - If the key name looks secret (see IsSecretKey) the value is replaced
//   with the literal string "[REDACTED]".
// - If the value is a string longer than 64 characters it's treated as a token
//   and redacted as well. Other values are returned unchanged.
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