package flow

import "strings"

// RedactionConfig controls which keys are treated as secrets and the maximum
// string length before a value is considered sensitive and redacted.
type RedactionConfig struct {
	Keys   map[string]struct{}
	MaxLen int
}

// WithRedactionConfig sets the App's redaction configuration by storing
// the provided config directly on the App. This is the preferred long-term
// storage location for per-App settings.
func WithRedactionConfig(keys []string, maxLen int) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		cfg := RedactionConfig{Keys: make(map[string]struct{}), MaxLen: maxLen}
		for _, k := range keys {
			cfg.Keys[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
		}
		a.redactionCfg = cfg
	}
}

// GetRedactionConfig returns the RedactionConfig for the given App if set,
// or nil otherwise.
// GetRedactionConfig returns the RedactionConfig for the given App if it
// has been explicitly configured via WithRedactionConfig. If no config was
// set this returns nil so callers fall back to package defaults.
func GetRedactionConfig(a *App) *RedactionConfig {
	if a == nil {
		return nil
	}
	if a.redactionCfg.Keys == nil && a.redactionCfg.MaxLen == 0 {
		return nil
	}
	return &a.redactionCfg
}

// RedactMapWithConfig applies redaction using the provided config. If cfg is nil
// it falls back to the package-level RedactMap behavior.
func RedactMapWithConfig(cfg *RedactionConfig, m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if cfg == nil {
		return RedactMap(m)
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		key := strings.ToLower(strings.TrimSpace(k))
		if _, ok := cfg.Keys[key]; ok {
			out[k] = "[REDACTED]"
			continue
		}
		if s, ok := v.(string); ok {
			if cfg.MaxLen > 0 && len(s) > cfg.MaxLen {
				out[k] = "[REDACTED]"
				continue
			}
		}
		out[k] = v
	}
	return out
}
