package flow

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewSessionManager_SecureByDefault verifies that a freshly constructed
// SessionManager uses Secure=true and SameSite=Lax without any extra calls.
func TestNewSessionManager_SecureByDefault(t *testing.T) {
	sm := NewSessionManager([]byte("secret"), "sess")
	if !sm.CookieSecure {
		t.Fatalf("expected CookieSecure=true by default, got false")
	}
	if sm.CookieSameSite != http.SameSiteLaxMode {
		t.Fatalf("expected CookieSameSite=Lax by default, got %v", sm.CookieSameSite)
	}
}

// TestNewSessionManager_CookieHasSecureFlag verifies that Set-Cookie from
// Save() actually contains the Secure attribute when default settings are used.
func TestNewSessionManager_CookieHasSecureFlag(t *testing.T) {
	sm := NewSessionManager([]byte("secret32byteslong______________"), "sess")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{values: map[string]interface{}{"k": "v"}, sm: sm, w: rr, r: req}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	setCookie := rr.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header, got none")
	}
	low := strings.ToLower(setCookie)
	if !strings.Contains(low, "secure") {
		t.Fatalf("Set-Cookie missing Secure flag: %q", setCookie)
	}
	if !strings.Contains(low, "samesite=lax") {
		t.Fatalf("Set-Cookie missing SameSite=Lax: %q", setCookie)
	}
}

// TestWithInsecureCookie_DisablesSecure verifies the escape hatch for local dev.
func TestWithInsecureCookie_DisablesSecure(t *testing.T) {
	sm := NewSessionManager([]byte("secret"), "sess").WithInsecureCookie()
	if sm.CookieSecure {
		t.Fatalf("WithInsecureCookie should set CookieSecure=false")
	}
	if sm.CookieSameSite != http.SameSiteDefaultMode {
		t.Fatalf("WithInsecureCookie should reset SameSite to Default, got %v", sm.CookieSameSite)
	}
}

// TestWithInsecureCookie_CookieLacksSecureFlag verifies insecure mode produces
// a cookie without the Secure attribute.
func TestWithInsecureCookie_CookieLacksSecureFlag(t *testing.T) {
	sm := NewSessionManager([]byte("secret32byteslong______________"), "sess").WithInsecureCookie()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s := &Session{values: map[string]interface{}{"k": "v"}, sm: sm, w: rr, r: req}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	setCookie := rr.Header().Get("Set-Cookie")
	low := strings.ToLower(setCookie)
	if strings.Contains(low, "secure") {
		t.Fatalf("WithInsecureCookie should not add Secure flag, got: %q", setCookie)
	}
}

// TestApplySecureCookieDefaults_IsNoOp verifies the deprecated method is
// backwards-compatible (does not panic, does not change anything).
func TestApplySecureCookieDefaults_IsNoOp(t *testing.T) {
	sm := NewSessionManager([]byte("secret"), "sess")
	sm.ApplySecureCookieDefaults() // should be a no-op
	if !sm.CookieSecure {
		t.Fatal("ApplySecureCookieDefaults must not break Secure=true default")
	}
}

// TestCookieStore_DeleteResponse_InheritsSecure verifies that deletion cookies
// from CookieStore carry the same Secure flag as the store's SessionManager.
func TestCookieStore_DeleteResponse_InheritsSecure(t *testing.T) {
	cs := NewCookieStore([]byte("secret32byteslong______________"), "sess", 3600)
	// Default should be secure
	if !cs.sm.CookieSecure {
		t.Fatal("CookieStore SessionManager should default to CookieSecure=true")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	if err := cs.DeleteResponse(rr, req, ""); err != nil {
		t.Fatalf("DeleteResponse: %v", err)
	}

	setCookie := rr.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie in DeleteResponse")
	}
	if !strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Fatalf("DeleteResponse cookie missing Secure flag: %q", setCookie)
	}
}

// TestRedisStoreAdapter_SecureByDefault verifies that NewRedisStoreAdapter
// defaults to Secure=true, SameSite=Lax.
func TestRedisStoreAdapter_SecureByDefault(t *testing.T) {
	rsa := NewRedisStoreAdapter([]byte("secret"), "sess", &RedisStore{})
	if !rsa.CookieSecure {
		t.Fatal("RedisStoreAdapter should default to CookieSecure=true")
	}
	if rsa.CookieSameSite != http.SameSiteLaxMode {
		t.Fatalf("RedisStoreAdapter should default to SameSite=Lax, got %v", rsa.CookieSameSite)
	}
}
