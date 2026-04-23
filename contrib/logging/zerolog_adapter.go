package logging

import (
	"context"

	"github.com/goflow-framework/flow/pkg/flow"
	"github.com/rs/zerolog"
)

// ZerologAdapter adapts zerolog.Logger to flow.StructuredLogger.
type ZerologAdapter struct {
	lg *zerolog.Logger
}

// NewZerologAdapter constructs an adapter. If l is nil returns nil.
func NewZerologAdapter(l *zerolog.Logger) flow.StructuredLogger {
	if l == nil {
		return nil
	}
	return &ZerologAdapter{lg: l}
}

func (z *ZerologAdapter) Log(level string, msg string, fields map[string]interface{}) {
	if z == nil || z.lg == nil {
		return
	}
	e := z.lg.With().Fields(fields).Logger()
	switch level {
	case "debug":
		e.Debug().Msg(msg)
	case "info":
		e.Info().Msg(msg)
	case "warn", "warning":
		e.Warn().Msg(msg)
	case "error":
		e.Error().Msg(msg)
	default:
		e.Info().Msg(msg)
	}
}

// Convenience helpers to satisfy StructuredLogger variants that include
// Debug/Info/Warn/Error helpers. These delegate to Log.
func (z *ZerologAdapter) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("debug", msg, kvToMap(keyvals))
}
func (z *ZerologAdapter) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("info", msg, kvToMap(keyvals))
}
func (z *ZerologAdapter) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("warn", msg, kvToMap(keyvals))
}
func (z *ZerologAdapter) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	z.Log("error", msg, kvToMap(keyvals))
}
