package logging

import (
	"context"
	"github.com/undiegomejia/flow/pkg/flow"
	"go.uber.org/zap"
)

// ZapAdapter adapts a *zap.Logger (or SugaredLogger) to flow.StructuredLogger.
type ZapAdapter struct {
	lg *zap.SugaredLogger
}

// NewZapAdapter creates an adapter from a *zap.Logger. If l is nil, nil is returned.
func NewZapAdapter(l *zap.Logger) flow.StructuredLogger {
	if l == nil {
		return nil
	}
	return &ZapAdapter{lg: l.Sugar()}
}

func (z *ZapAdapter) Log(level string, msg string, fields map[string]interface{}) {
	if z == nil || z.lg == nil {
		return
	}
	switch level {
	case "debug":
		z.lg.Debugw(msg, convertFields(fields)...)
	case "info":
		z.lg.Infow(msg, convertFields(fields)...)
	case "warn", "warning":
		z.lg.Warnw(msg, convertFields(fields)...)
	case "error":
		z.lg.Errorw(msg, convertFields(fields)...)
	default:
		z.lg.Infow(msg, convertFields(fields)...)
	}
}

func convertFields(m map[string]interface{}) []interface{} {
	if len(m) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(m)*2)
	for k, v := range m {
		out = append(out, k, v)
	}
	return out
}

// Convenience helpers to satisfy StructuredLogger variants that include
// Debug/Info/Warn/Error helpers. These delegate to Log and use the shared kvToMap helper.
func (z *ZapAdapter) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("debug", msg, kvToMap(keyvals))
}
func (z *ZapAdapter) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("info", msg, kvToMap(keyvals))
}
func (z *ZapAdapter) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("warn", msg, kvToMap(keyvals))
}
func (z *ZapAdapter) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("error", msg, kvToMap(keyvals))
}
