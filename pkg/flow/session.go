package flow

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// SessionManager handles encoding/decoding sessions into a signed, encrypted
// cookie. The cookie payload is opaque to the client: the JSON is encrypted
// with AES-256-GCM before being base64-encoded, so session values (user IDs,
// roles, etc.) cannot be read or tampered with without the secret.
//
// Wire format:  base64url(nonce || ciphertext) | hex(HMAC-SHA256)
//
//   - The HMAC covers the base64url-encoded ciphertext blob so any
//     modification of either part is detected.
//   - AES and HMAC keys are derived independently from the single secret
//     using HKDF-SHA256, so the same bytes serve both purposes safely.
//
// Backward compatibility note: cookies written by the old (sign-only) format
// are rejected on load because they will fail HMAC verification under the new
// key derivation; the session is treated as absent and a fresh one starts.
type SessionManager struct {
	secret     []byte
	aeadKey    []byte // 32-byte key for AES-256-GCM
	macKey     []byte // 32-byte key for HMAC-SHA256 authentication
	CookieName string
	// MaxAge in seconds.
	MaxAge int
	// CookieSecure, when true, marks session cookies with the Secure flag.
	CookieSecure bool
	// CookieSameSite controls the SameSite attribute for session cookies.
	CookieSameSite http.SameSite
}

// NewSessionManager constructs a manager with the provided secret. The secret
// must be at least 32 bytes; longer secrets are accepted. If cookieName is
// empty, "flow_session" is used.
//
// Two independent 32-byte sub-keys are derived from the secret:
//   - an AES-256-GCM encryption key (HKDF info = "flow-session-enc")
//   - an HMAC-SHA256 authentication key (HKDF info = "flow-session-mac")
func NewSessionManager(secret []byte, cookieName string) *SessionManager {
	if cookieName == "" {
		cookieName = "flow_session"
	}
	aeadKey := deriveKey(secret, "flow-session-enc")
	macKey := deriveKey(secret, "flow-session-mac")
	return &SessionManager{
		secret:         secret,
		aeadKey:        aeadKey,
		macKey:         macKey,
		CookieName:     cookieName,
		MaxAge:         86400,
		CookieSecure:   true,
		CookieSameSite: http.SameSiteLaxMode,
	}
}

// deriveKey produces a 32-byte sub-key from secret using a single HKDF-style
// HMAC step: HMAC-SHA256(secret, info). This is intentionally simple and
// avoids an external dependency; for production hardening you can replace this
// with golang.org/x/crypto/hkdf without changing the public API.
func deriveKey(secret []byte, info string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(info))
	return mac.Sum(nil) // 32 bytes
}

// generateRandomSecret returns n random bytes suitable for use as a secret.
func generateRandomSecret(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// loadFromRequest decodes and decrypts session data from the request cookie.
// If the cookie is absent, invalid, or tampered with, an empty session map
// is returned (not an error) so the caller always gets a usable session.
func (sm *SessionManager) loadFromRequest(r *http.Request) (map[string]interface{}, error) {
	c, err := r.Cookie(sm.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}

	parts := strings.SplitN(c.Value, "|", 2)
	if len(parts) != 2 {
		return map[string]interface{}{}, nil
	}

	// Verify HMAC before decrypting (authenticate-then-decrypt).
	cipherBlob, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return map[string]interface{}{}, nil
	}
	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return map[string]interface{}{}, nil
	}
	mac := hmac.New(sha256.New, sm.macKey)
	mac.Write([]byte(parts[0])) // MAC covers the base64url-encoded blob
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return map[string]interface{}{}, nil
	}

	// Decrypt the ciphertext blob (nonce || ciphertext).
	plaintext, err := sm.decryptAESGCM(cipherBlob)
	if err != nil {
		return map[string]interface{}{}, nil
	}

	var val map[string]interface{}
	if err := json.Unmarshal(plaintext, &val); err != nil {
		return map[string]interface{}{}, nil
	}
	return val, nil
}

// encodeForCookie encrypts the session map with AES-256-GCM and signs the
// result with HMAC-SHA256. The returned string is safe to store in a cookie.
func (sm *SessionManager) encodeForCookie(values map[string]interface{}) (string, error) {
	plaintext, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	cipherBlob, err := sm.encryptAESGCM(plaintext)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(cipherBlob)

	mac := hmac.New(sha256.New, sm.macKey)
	mac.Write([]byte(encoded))
	sig := hex.EncodeToString(mac.Sum(nil))

	return encoded + "|" + sig, nil
}

// encryptAESGCM encrypts plaintext using AES-256-GCM.
// Returns nonce || ciphertext as a single byte slice.
func (sm *SessionManager) encryptAESGCM(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(sm.aeadKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// Seal appends ciphertext+tag after nonce.
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptAESGCM decrypts a nonce||ciphertext blob produced by encryptAESGCM.
func (sm *SessionManager) decryptAESGCM(blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(sm.aeadKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(blob) < nonceSize {
		return nil, errors.New("session: ciphertext too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, nil)
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

// sessionCtxKey is the context key used to attach the session to requests.
type sessionCtxKey struct{}

// Middleware returns a flow Middleware that loads the session into the request
// context so handlers can call FromContext to retrieve it.
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
// convenient for development but a stable secret must be configured in
// production, otherwise sessions are invalidated on every restart.
func DefaultSessionManager() *SessionManager {
	s, _ := generateRandomSecret(32)
	return NewSessionManager(s, "flow_session")
}

// ApplySecureCookieDefaults is a no-op kept for backwards compatibility.
// Secure=true and SameSite=Lax are now the defaults set by NewSessionManager.
//
// Deprecated: safe to remove from call-sites; the defaults already provide
// the same behaviour.
func (sm *SessionManager) ApplySecureCookieDefaults() {}

// WithInsecureCookie disables the Secure flag on session cookies and resets
// SameSite to the browser default. Use this only in local development or
// test environments that do not serve over HTTPS.
func (sm *SessionManager) WithInsecureCookie() *SessionManager {
	sm.CookieSecure = false
	sm.CookieSameSite = http.SameSiteDefaultMode
	return sm
}
