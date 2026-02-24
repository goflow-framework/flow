package flow

import (
	"strings"
	"sync"
)

// RedactionConfig controls which keys are treated as secrets and the maximum
// string length before a value is considered sensitive and redacted.
type RedactionConfig struct {
	Keys   map[string]struct{}
	MaxLen int
}

// WithRedactionConfig sets the App's redaction configuration.
var (
	appRedactionMu  sync.RWMutex
	appRedactionCfg = make(map[*App]RedactionConfig)
)

// WithRedactionConfig sets the App's redaction configuration.
// The configuration is stored in a package-level map keyed by the App
// pointer to avoid modifying the App struct; helpers can retrieve it with
// GetRedactionConfig.
func WithRedactionConfig(keys []string, maxLen int) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		cfg := RedactionConfig{Keys: make(map[string]struct{}), MaxLen: maxLen}
		for _, k := range keys {
			cfg.Keys[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
		}
		appRedactionMu.Lock()
		appRedactionCfg[a] = cfg
		appRedactionMu.Unlock()
	}
}

// GetRedactionConfig returns the RedactionConfig for the given App if set,
// or nil otherwise.
func GetRedactionConfig(a *App) *RedactionConfig {
	if a == nil {
		return nil
	}
	appRedactionMu.RLock()
	defer appRedactionMu.RUnlock()
	if cfg, ok := appRedactionCfg[a]; ok {
		return &cfg
	}
	return nil
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
