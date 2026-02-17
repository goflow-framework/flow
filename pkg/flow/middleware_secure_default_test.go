package flow

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWithDefaultMiddleware_IncludesSecureHeaders ensures that constructing an
// App with WithDefaultMiddleware registers the SecureHeaders middleware so
// responses include conservative security headers such as X-Frame-Options
// and X-Content-Type-Options.
func TestWithDefaultMiddleware_IncludesSecureHeaders(t *testing.T) {
	app := New("test-app", WithDefaultMiddleware())

	// Handler intentionally does nothing — middleware should inject headers.
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.local/", nil)
	app.Handler().ServeHTTP(rr, req)

	// SecureHeaders sets X-Frame-Options and X-Content-Type-Options unconditionally.
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got == "" {
		t.Fatalf("expected Referrer-Policy to be set")
	}
}
