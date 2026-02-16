package flow

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// SessionManager handles encoding/decoding sessions into a signed cookie.
// It's intentionally small and dependency-free for the prototype.
type SessionManager struct {
	secret     []byte
	CookieName string
	// MaxAge in seconds
	MaxAge int
	// CookieSecure, when true, will mark session cookies with Secure flag.
	CookieSecure bool
	// CookieSameSite controls the SameSite attribute for session cookies.
	CookieSameSite http.SameSite
}

// NewSessionManager constructs a manager with the provided secret. If
// cookieName is empty, a default is used.
func NewSessionManager(secret []byte, cookieName string) *SessionManager {
	if cookieName == "" {
		cookieName = "flow_session"
	}
	// Secure-by-default: set conservative cookie attributes suitable for
	// production HTTPS deployments. Callers may opt out by setting
	// CookieSecure=false or changing CookieSameSite after construction.
	return &SessionManager{secret: secret, CookieName: cookieName, MaxAge: 86400, CookieSecure: true, CookieSameSite: http.SameSiteLaxMode}
}

// generateRandomSecret returns n bytes of randomness.
func generateRandomSecret(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// loadFromRequest decodes session data from request cookie. If invalid or
// absent, returns an empty session map.
func (sm *SessionManager) loadFromRequest(r *http.Request) (map[string]interface{}, error) {
	c, err := r.Cookie(sm.CookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}
	parts := strings.Split(c.Value, "|")
	if len(parts) != 2 {
		return map[string]interface{}{}, nil
	}
	dataB, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return map[string]interface{}{}, nil
	}
	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return map[string]interface{}{}, nil
	}
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write(dataB)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return map[string]interface{}{}, nil
	}
	var val map[string]interface{}
	if err := json.Unmarshal(dataB, &val); err != nil {
		return map[string]interface{}{}, nil
	}
	return val, nil
}

// encodeForCookie serializes the map and signs it.
func (sm *SessionManager) encodeForCookie(values map[string]interface{}) (string, error) {
	b, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write(b)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(b) + "|" + hex.EncodeToString(sig), nil
}

// Session represents a request-scoped session. It is safe to modify and
// Save will encode it back to a cookie on the response.
type Session struct {
	values map[string]interface{}
	sm     *SessionManager
	w      http.ResponseWriter
	r      *http.Request
}

// Get returns a value from the session.
func (s *Session) Get(key string) (interface{}, bool) {
	v, ok := s.values[key]
	return v, ok
}

// Set stores a value in the session and writes the cookie immediately.
func (s *Session) Set(key string, v interface{}) error {
	s.values[key] = v
	return s.Save()
}

// Delete removes a key and saves.
func (s *Session) Delete(key string) error {
	delete(s.values, key)
	return s.Save()
}

// Save encodes the session and sets the cookie.
func (s *Session) Save() error {
	enc, err := s.sm.encodeForCookie(s.values)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     s.sm.CookieName,
		Value:    enc,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.sm.CookieSecure,
		SameSite: s.sm.CookieSameSite,
		Expires:  time.Now().Add(time.Duration(s.sm.MaxAge) * time.Second),
		MaxAge:   s.sm.MaxAge,
	}
	http.SetCookie(s.w, cookie)
	return nil
}

// sessionCtxKey is the context key used to attach session to requests.
type sessionCtxKey struct{}

// Middleware returns a flow Middleware that loads session into request
// context and exposes a Session for handlers to use.
func (sm *SessionManager) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vals, _ := sm.loadFromRequest(r)
			s := &Session{values: vals, sm: sm, w: w, r: r}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext extracts the Session from context or returns nil.
func FromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(sessionCtxKey{}).(*Session); ok {
		return v
	}
	return nil
}

// DefaultSessionManager constructs a manager with a random secret. It is
// convenient for development but should be configured in production.
func DefaultSessionManager() *SessionManager {
	s, _ := generateRandomSecret(32)
	return NewSessionManager(s, "flow_session")
}

// ApplySecureCookieDefaults enables conservative cookie flags for session
// cookies: Secure=true, HttpOnly is already set by Save(), and
// SameSite=Lax. Call this in app initialization when serving over HTTPS.
func (sm *SessionManager) ApplySecureCookieDefaults() {
	sm.CookieSecure = true
	sm.CookieSameSite = http.SameSiteLaxMode
}
