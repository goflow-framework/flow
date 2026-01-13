package observability

import "strings"

// ParseHeaders parses a comma-separated list of key=val pairs into a map.
// Example: "api-key=foo,env=dev" -> map[string]string{"api-key":"foo","env":"dev"}
// Malformed entries are ignored.
func ParseHeaders(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if k == "" {
			continue
		}
		m[k] = v
	}
	return m
}
