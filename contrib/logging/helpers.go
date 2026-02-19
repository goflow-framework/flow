package logging

import "context"

// kvToMap converts a variadic key/value slice into a map[string]interface{}.
// It ignores non-string keys and odd-length slices.
func kvToMap(kv []interface{}) map[string]interface{} {
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

// The adapters' convenience helpers accept a context.Context to match the
// flow.StructuredLogger helper signatures. The context is intentionally unused
// here but included for interface compatibility.
var _ = context.Background
