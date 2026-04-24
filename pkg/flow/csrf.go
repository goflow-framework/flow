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
// requests. It requires SessionManager.Middleware() to be registered earlier
// in the stack: if no session is found in the request context the middleware
// responds with 500 Internal Server Error and a diagnostic message rather than
// silently issuing a per-request token that can never be validated (which
// would cause every state-changing request to return 403 with no explanation).
//
// If the OS entropy pool is unavailable while generating a new token the
// middleware also responds with 500 Internal Server Error rather than issuing
// a predictable (all-zero) token that would silently degrade CSRF protection.
func CSRFMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Guard: session middleware must be registered before CSRFMiddleware.
			// Without a session the token cannot be persisted between requests,
			// so every unsafe request would silently return 403. Fail loudly
			// instead so the misconfiguration surfaces on the very first request.
			sess := FromContext(r.Context())
			if sess == nil {
				http.Error(w,
					"csrf: session middleware is not configured — "+
						"register SessionManager.Middleware() before CSRFMiddleware() in your middleware stack",
					http.StatusInternalServerError)
				return
			}

			// ensure token exists in the session
			var token string
			if v, ok := sess.Get(csrfSessionKey); ok {
				if s, ok := v.(string); ok {
					token = s
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
				// use Set which also persists via Save
				_ = sess.Set(csrfSessionKey, token)
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
} // CSRFToken returns the CSRF token for the request's session, or empty string.
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
