package zapadapter

import (
	"context"

	"go.uber.org/zap"

	"github.com/undiegomejia/flow/pkg/flow"
)

// ZapAdapter adapts an *zap.Logger to the framework's StructuredLogger
// interface. Use NewZapAdapter to obtain a *flow.LoggerAdapter which can be
// passed to App via WithLogger or used directly where a flow.Logger is
// expected.
type ZapAdapter struct {
	L *zap.Logger
}

// NewZapAdapter returns a *flow.LoggerAdapter wrapping the provided zap
// logger. If nil is provided a noop zap logger is used.
func NewZapAdapter(z *zap.Logger) *flow.LoggerAdapter {
	if z == nil {
		z = zap.NewNop()
	}
	return &flow.LoggerAdapter{L: &ZapAdapter{L: z}}
}

func (z *ZapAdapter) Log(level string, msg string, fields map[string]interface{}) {
	if z == nil || z.L == nil {
		return
	}
	f := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		f = append(f, zap.Any(k, v))
	}
	switch level {
	case "debug":
		z.L.Debug(msg, f...)
	case "warn", "warning":
		z.L.Warn(msg, f...)
	case "error":
		z.L.Error(msg, f...)
	default:
		z.L.Info(msg, f...)
	}
}

func (z *ZapAdapter) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	z.logWithKeyvals("debug", msg, keyvals...)
}
func (z *ZapAdapter) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	z.logWithKeyvals("info", msg, keyvals...)
}
func (z *ZapAdapter) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	z.logWithKeyvals("warn", msg, keyvals...)
}
func (z *ZapAdapter) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	z.logWithKeyvals("error", msg, keyvals...)
}

// logWithKeyvals converts variadic key/value pairs into zap fields and logs
// at the provided level.
func (z *ZapAdapter) logWithKeyvals(level, msg string, keyvals ...interface{}) {
	if z == nil || z.L == nil {
		return
	}
	f := make([]zap.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals)-1; i += 2 {
		k, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		f = append(f, zap.Any(k, keyvals[i+1]))
	}
	switch level {
	case "debug":
		z.L.Debug(msg, f...)
	case "warn", "warning":
		z.L.Warn(msg, f...)
	case "error":
		z.L.Error(msg, f...)
	default:
		z.L.Info(msg, f...)
	}
}
