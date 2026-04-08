package flow

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

const csrfSessionKey = "_csrf_token"

// csrfRandReader is the source of randomness used by generateCSRFToken.
// It is a package-level variable so tests can swap it for a failing reader
// to exercise the error path without touching crypto/rand.
var csrfRandReader io.Reader = rand.Reader

// CSRFMiddleware ensures a per-session CSRF token exists and validates unsafe
// requests. If the OS entropy pool is unavailable while generating a new token
// the middleware responds with 500 Internal Server Error rather than issuing a
// predictable (all-zero) token that would silently degrade CSRF protection.
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
				var err error
				token, err = generateCSRFToken()
				if err != nil {
					// crypto/rand failure — do NOT issue a predictable token.
					// Serving 500 is safer than degrading CSRF protection.
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}
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

// generateCSRFToken returns a cryptographically random 32-byte token encoded
// as base64url, or an error if the OS entropy source is unavailable.
// It reads from csrfRandReader so tests can inject a failing reader.
func generateCSRFToken() (string, error) {
	var b [32]byte
	if _, err := io.ReadFull(csrfRandReader, b[:]); err != nil {
		return "", fmt.Errorf("csrf: rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
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
