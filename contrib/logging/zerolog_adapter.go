package logging

import (
    "github.com/undiegomejia/flow/pkg/flow"
    "github.com/rs/zerolog"
)

// ZerologAdapter adapts zerolog.Logger to flow.StructuredLogger.
type ZerologAdapter struct{
    lg *zerolog.Logger
}

// NewZerologAdapter constructs an adapter. If l is nil returns nil.
func NewZerologAdapter(l *zerolog.Logger) flow.StructuredLogger {
    if l == nil { return nil }
    return &ZerologAdapter{lg: l}
}

func (z *ZerologAdapter) Log(level string, msg string, fields map[string]interface{}) {
    if z == nil || z.lg == nil { return }
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

// Provide the Debug/Info/Warn/Error helpers to satisfy variants.
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

func kvToMap(kv []interface{}) map[string]interface{} {
    if len(kv) == 0 { return nil }
    m := make(map[string]interface{}, len(kv)/2)
    for i := 0; i < len(kv)-1; i += 2 {
        k, ok := kv[i].(string)
        if !ok { continue }
        m[k] = kv[i+1]
    }
    return m
}
package logging

import (
    "github.com/undiegomejia/flow/pkg/flow"
    "github.com/rs/zerolog"
)

// ZerologAdapter adapts zerolog.Logger to flow.StructuredLogger.
type ZerologAdapter struct{
    lg *zerolog.Logger
}

// NewZerologAdapter constructs an adapter. If l is nil returns nil.
func NewZerologAdapter(l *zerolog.Logger) flow.StructuredLogger {
    if l == nil { return nil }
    return &ZerologAdapter{lg: l}
}

func (z *ZerologAdapter) Log(level string, msg string, fields map[string]interface{}) {
    if z == nil || z.lg == nil { return }
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
