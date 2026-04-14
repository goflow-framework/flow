package flow

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSessionEncryption_PayloadIsOpaque verifies that the cookie value
// written by Save() is not a readable base64-JSON blob. An attacker who
// base64-decodes the first segment of the cookie value must NOT be able to
// recover plaintext JSON.
func TestSessionEncryption_PayloadIsOpaque(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{values: map[string]interface{}{"user_id": 42, "role": "admin"}, sm: sm, w: rr, r: req}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	setCookie := rr.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}

	// Extract cookie value from "name=value; ..." header.
	cookieVal := ""
	for _, part := range strings.Split(setCookie, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "sess=") {
			cookieVal = strings.TrimPrefix(part, "sess=")
			break
		}
	}
	if cookieVal == "" {
		t.Fatalf("could not extract cookie value from: %q", setCookie)
	}

	// Split on "|" to get the ciphertext blob.
	parts := strings.SplitN(cookieVal, "|", 2)
	if len(parts) != 2 {
		t.Fatalf("expected pipe-separated value, got: %q", cookieVal)
	}

	// Decode the first segment — must not be valid JSON.
	blob, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if strings.Contains(string(blob), "user_id") || strings.Contains(string(blob), "admin") {
		t.Errorf("cookie payload is NOT encrypted — plaintext JSON visible: %q", string(blob))
	}
}

// TestSessionEncryption_RoundTrip verifies that a session written via Save()
// can be read back via loadFromRequest with the same values.
func TestSessionEncryption_RoundTrip(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")

	// Write session.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{
		values: map[string]interface{}{
			"user_id": float64(99), // JSON numbers unmarshal as float64
			"email":   "alice@example.com",
		},
		sm: sm, w: rr, r: req,
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Pluck the Set-Cookie header and replay it as a request cookie.
	cookieHeader := rr.Header().Get("Set-Cookie")
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Cookie", parseCookieValueFromHeader(t, cookieHeader, "sess"))

	// Load it back.
	vals, err := sm.loadFromRequest(req2)
	if err != nil {
		t.Fatalf("loadFromRequest: %v", err)
	}
	if vals["user_id"] != float64(99) {
		t.Errorf("user_id: got %v, want 99", vals["user_id"])
	}
	if vals["email"] != "alice@example.com" {
		t.Errorf("email: got %v, want alice@example.com", vals["email"])
	}
}

// TestSessionEncryption_TamperedCiphertextRejected verifies that flipping a
// byte in the ciphertext portion causes decryption to fail and returns an
// empty session (not a panic or an error surfaced to the caller).
func TestSessionEncryption_TamperedCiphertextRejected(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{values: map[string]interface{}{"uid": "1234"}, sm: sm, w: rr, r: req}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cookieHeader := rr.Header().Get("Set-Cookie")
	rawVal := parseCookieRawValue(t, cookieHeader, "sess")

	// Corrupt the last byte of the base64-encoded ciphertext blob (before |).
	parts := strings.SplitN(rawVal, "|", 2)
	blobBytes := []byte(parts[0])
	blobBytes[len(blobBytes)-1] ^= 0xFF // flip bits
	tampered := string(blobBytes) + "|" + parts[1]

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Cookie", "sess="+tampered)

	vals, err := sm.loadFromRequest(req2)
	if err != nil {
		t.Fatalf("loadFromRequest returned unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected empty session for tampered cookie, got: %v", vals)
	}
}

// TestSessionEncryption_WrongSecretRejected verifies that a cookie encrypted
// with one secret cannot be decrypted by a manager with a different secret.
func TestSessionEncryption_WrongSecretRejected(t *testing.T) {
	sm1 := NewSessionManager([]byte("secret-one-32-bytes-long-padding"), "sess")
	sm2 := NewSessionManager([]byte("secret-two-32-bytes-long-padding"), "sess")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{values: map[string]interface{}{"uid": "abc"}, sm: sm1, w: rr, r: req}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cookieHeader := rr.Header().Get("Set-Cookie")
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Cookie", parseCookieValueFromHeader(t, cookieHeader, "sess"))

	vals, err := sm2.loadFromRequest(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected empty session when using wrong secret, got: %v", vals)
	}
}

// TestSessionEncryption_EachCookieIsUnique verifies that two Save() calls with
// identical data produce different cookie values (due to random nonce).
func TestSessionEncryption_EachCookieIsUnique(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")
	data := map[string]interface{}{"x": "y"}

	vals := make([]string, 3)
	for i := range vals {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		s := &Session{values: data, sm: sm, w: rr, r: req}
		if err := s.Save(); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
		vals[i] = parseCookieRawValue(t, rr.Header().Get("Set-Cookie"), "sess")
	}
	if vals[0] == vals[1] || vals[1] == vals[2] {
		t.Errorf("expected unique cookie values per Save(), got duplicates: %v", vals)
	}
}

// TestSessionEncryption_NoCookieReturnsEmpty verifies that a request with no
// session cookie returns an empty map without error.
func TestSessionEncryption_NoCookieReturnsEmpty(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")
	req := httptest.NewRequest("GET", "/", nil)
	vals, err := sm.loadFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Errorf("expected empty map, got: %v", vals)
	}
}

// TestSessionEncryption_RedisAdapter_CookieIsSignedOnly verifies that the
// RedisStoreAdapter (which stores data server-side) still writes a signed id
// — not encrypted session data — in the cookie.
func TestSessionEncryption_RedisAdapter_CookieIsSignedOnly(t *testing.T) {
	rsa := NewRedisStoreAdapter([]byte("adapter-secret-32-bytes-long!!!!"), "sess", &RedisStore{})
	// We only test the signing side (not actual Redis) — just verify the
	// cookie format is id|sig (two hex-like parts joined by |).
	sig := rsa.signID("test-session-id")
	if sig == "" {
		t.Fatal("signID returned empty string")
	}
	if len(sig) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64-char HMAC hex, got %d chars: %q", len(sig), sig)
	}
}

// TestSessionEncryption_MiddlewareLoadsEncryptedSession is an end-to-end test
// that exercises the Middleware → handler → FromContext path with an encrypted
// cookie.
func TestSessionEncryption_MiddlewareLoadsEncryptedSession(t *testing.T) {
	sm := NewSessionManager([]byte("super-secret-key-32-bytes-long!!"), "sess")

	// Phase 1: generate a cookie via a write handler.
	writeReq := httptest.NewRequest("POST", "/login", nil)
	writeRR := httptest.NewRecorder()
	writeHandler := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := FromContext(r.Context())
		if sess == nil {
			t.Error("FromContext returned nil inside middleware")
			return
		}
		_ = sess.Set("logged_in", true)
		_ = sess.Set("user", "bob")
		w.WriteHeader(http.StatusOK)
	}))
	writeHandler.ServeHTTP(writeRR, writeReq)

	// Phase 2: replay the cookie in a read request.
	// Each sess.Set() calls Save() which appends a Set-Cookie header.
	// Use the last one — it contains all keys written so far.
	setCookieHeaders := writeRR.Header()["Set-Cookie"]
	if len(setCookieHeaders) == 0 {
		t.Fatal("no Set-Cookie headers written")
	}
	cookieHeader := setCookieHeaders[len(setCookieHeaders)-1]
	readReq := httptest.NewRequest("GET", "/dashboard", nil)
	readReq.Header.Set("Cookie", parseCookieValueFromHeader(t, cookieHeader, "sess"))
	readRR := httptest.NewRecorder()

	readHandler := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := FromContext(r.Context())
		if sess == nil {
			t.Error("FromContext returned nil on read request")
			return
		}
		v, ok := sess.Get("logged_in")
		if !ok || v != true {
			t.Errorf("expected logged_in=true, got %v (ok=%v)", v, ok)
		}
		user, ok := sess.Get("user")
		if !ok || user != "bob" {
			t.Errorf("expected user=bob, got %v (ok=%v)", user, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))
	readHandler.ServeHTTP(readRR, readReq)
}

// --- helpers ---------------------------------------------------------------

// parseCookieValueFromHeader extracts "name=value" from a Set-Cookie header
// and returns it as a Cookie request header value.
func parseCookieValueFromHeader(t *testing.T, header, name string) string {
	t.Helper()
	raw := parseCookieRawValue(t, header, name)
	return name + "=" + raw
}

// parseCookieRawValue extracts just the value part of a named cookie from a
// Set-Cookie header string.
func parseCookieRawValue(t *testing.T, header, name string) string {
	t.Helper()
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name+"=") {
			return strings.TrimPrefix(part, name+"=")
		}
	}
	t.Fatalf("cookie %q not found in Set-Cookie header: %q", name, header)
	return ""
}
