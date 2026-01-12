package flow

import (
    "crypto/rand"
    "encoding/hex"
    "net/http"
    "strings"
)

// CSRF middleware uses a per-session token stored under the key "_csrf".
// For unsafe methods it expects the client to send the token in the
// X-CSRF-Token header (double-submit pattern is also possible).
func CSRFMiddleware() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // retrieve session
            s := FromContext(r.Context())
            if s == nil {
                // no session available — continue (handlers can opt-in)
                next.ServeHTTP(w, r)
                return
            }
            // ensure token exists
            tokI, ok := s.Get("_csrf")
            var token string
            if !ok {
                b := make([]byte, 16)
                if _, err := rand.Read(b); err == nil {
                    token = hex.EncodeToString(b)
                    _ = s.Set("_csrf", token)
                }
            } else {
                if ts, ok := tokI.(string); ok {
                    token = ts
                }
            }

            // verify for unsafe methods
            if isUnsafeMethod(r.Method) {
                // check header
                hdr := r.Header.Get("X-CSRF-Token")
                if hdr == "" || hdr != token {
                    http.Error(w, "invalid csrf token", http.StatusForbidden)
                    return
                }
            }

            // attach helper header with token for convenience
            if token != "" {
                w.Header().Set("X-CSRF-Token", token)
            }

            next.ServeHTTP(w, r)
        })
    }
}

func isUnsafeMethod(m string) bool {
    m = strings.ToUpper(m)
    return m == "POST" || m == "PUT" || m == "PATCH" || m == "DELETE"
}
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
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ensure token exists
			sess := FromContext(r.Context())
			var token string
			if sess != nil {
				if v, ok := sess.values[csrfSessionKey].(string); ok && v != "" {
					token = v
				}
			}
			if token == "" {
				token = generateCSRFToken()
				if sess != nil {
					sess.values[csrfSessionKey] = token
					// persist session on response
					sess.Save()
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
		if v, ok := s.values[csrfSessionKey].(string); ok {
			return v
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
