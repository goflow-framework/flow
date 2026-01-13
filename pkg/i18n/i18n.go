package i18n

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manager holds loaded translations by locale (eg. "en", "es").
type Manager struct {
	translations map[string]map[string]string
}

// NewManager constructs an empty Manager.
func NewManager() *Manager {
	return &Manager{translations: make(map[string]map[string]string)}
}

// LoadDir loads all .yml/.yaml files in dir. Files should be named by locale
// (eg. en.yaml, es.yaml) and may contain nested maps. Keys will be flattened
// using dot notation (eg. admin.posts.index).
func (m *Manager) LoadDir(dir string) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		locale := strings.TrimSuffix(name, ext)
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var raw map[string]interface{}
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return fmt.Errorf("yaml unmarshal %s: %w", name, err)
		}
		flat := make(map[string]string)
		flattenMap("", raw, flat)
		m.translations[locale] = flat
	}
	return nil
}

func flattenMap(prefix string, v interface{}, out map[string]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, vv := range t {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenMap(key, vv, out)
		}
	case string:
		out[prefix] = t
	default:
		out[prefix] = fmt.Sprintf("%v", t)
	}
}

// Get returns the translation for locale/key if present.
func (m *Manager) Get(locale, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	if tm, ok := m.translations[locale]; ok {
		if s, ok2 := tm[key]; ok2 {
			return s, true
		}
	}
	return "", false
}

// context key and value
type ctxKey struct{}

type ctxVal struct {
	Mgr    *Manager
	Locale string
}

// Middleware returns an http middleware that selects a locale for the request
// and stores the Manager+locale into the request context. Selection order:
// query param `lang`, cookie `lang`, Accept-Language, defaultLocale.
func Middleware(mgr *Manager, defaultLocale string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := defaultLocale
			if q := r.URL.Query().Get("lang"); q != "" {
				locale = q
			} else if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
				locale = c.Value
			} else if al := r.Header.Get("Accept-Language"); al != "" {
				parts := strings.Split(al, ",")
				if len(parts) > 0 {
					p := strings.TrimSpace(parts[0])
					// trim region if present (en-US -> en)
					locale = strings.SplitN(p, "-", 2)[0]
				}
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, ctxVal{Mgr: mgr, Locale: locale})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext retrieves the manager and locale stored by the middleware.
func FromContext(ctx context.Context) (*Manager, string, bool) {
	if v := ctx.Value(ctxKey{}); v != nil {
		if cv, ok := v.(ctxVal); ok {
			return cv.Mgr, cv.Locale, true
		}
	}
	return nil, "", false
}

// TFromContext returns the translation for key using the Manager/locale in
// ctx. If formatting args are provided, the translation is used as a
// fmt.Sprintf format string. If translation is missing, the key is returned.
func TFromContext(ctx context.Context, key string, args ...interface{}) string {
	mgr, locale, ok := FromContext(ctx)
	if !ok || mgr == nil {
		if len(args) > 0 {
			return fmt.Sprintf(key, args...)
		}
		return key
	}
	if s, ok := mgr.Get(locale, key); ok {
		if len(args) > 0 {
			return fmt.Sprintf(s, args...)
		}
		return s
	}
	// fallback: try default locale "en"
	if s, ok := mgr.Get("en", key); ok {
		if len(args) > 0 {
			return fmt.Sprintf(s, args...)
		}
		return s
	}
	if len(args) > 0 {
		return fmt.Sprintf(key, args...)
	}
	return key
}
