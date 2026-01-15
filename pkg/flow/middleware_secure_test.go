package flow

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecureHeaders_Defaults_TLSRequest(t *testing.T) {
	app := New("secure-test")
	// register secure defaults
	if err := WithSecureDefaults(app); err != nil {
		t.Fatalf("WithSecureDefaults: %v", err)
	}

	// build handler that returns 204
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))

	// create a request and mark it as TLS
	req := httptest.NewRequest("GET", "https://example.local/", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	// HSTS should be present
	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Fatalf("expected Strict-Transport-Security header, got empty")
	}

	// Common headers
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("unexpected X-Frame-Options: %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", got)
	}
}

func TestSecureHeaders_NoHSTSForPlainHTTP(t *testing.T) {
	app := New("secure-test-plain")
	if err := WithSecureDefaults(app); err != nil {
		t.Fatalf("WithSecureDefaults: %v", err)
	}
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	req := httptest.NewRequest("GET", "http://example.local/", nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS on plain http, got %q", got)
	}
}

func TestSecureHeaders_OptionOverride(t *testing.T) {
	// custom options
	opt := func(o *SecureHeadersOptions) {
		o.HSTSMaxAge = 24 * time.Hour
		o.HSTSIncludeSubdomains = true
		o.ContentSecurityPolicy = "default-src 'self'"
	}

	app := New("secure-opts")
	app.Use(SecureHeaders(opt))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	req := httptest.NewRequest("GET", "https://example.local/", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Fatalf("unexpected CSP: %q", got)
	}
	if got := rr.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatalf("expected HSTS header with custom max-age")
	}
}
