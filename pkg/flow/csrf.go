package flow

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"net/http"
)

const csrfSessionKey = "_csrf_token"

// CSRFMiddleware ensures a per-session CSRF token exists and validates unsafe requests.
func CSRFMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ensure token exists
			sess := FromContext(r.Context())
			var token string
			if sess != nil {
				if v, ok := sess.Get(csrfSessionKey); ok {
					if s, ok := v.(string); ok {
						token = s
					}
				}
			}
			if token == "" {
				token = generateCSRFToken()
				if sess != nil {
					// use Set which also persists via Save
					_ = sess.Set(csrfSessionKey, token)
				}
			}

			// Validate unsafe methods
			if isUnsafeMethod(r.Method) {
				// check header first
				header := r.Header.Get("X-CSRF-Token")
				if header == "" {
					// also check form field
					if err := r.ParseForm(); err == nil {
						header = r.Form.Get("_csrf")
					}
				}
				if !secureCompare(header, token) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFToken returns the CSRF token for the request's session, or empty string.
func CSRFToken(r *http.Request) string {
	if s := FromContext(r.Context()); s != nil {
		if v, ok := s.Get(csrfSessionKey); ok {
			if st, ok := v.(string); ok {
				return st
			}
		}
	}
	return ""
}

func generateCSRFToken() string {
	var b [32]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		// fallback to empty on error
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func secureCompare(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func isUnsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
