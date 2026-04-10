package flow

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// hasJSONContentType unit tests
// ---------------------------------------------------------------------------

func TestHasJSONContentType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ct   string
		want bool
		desc string
	}{
		// Standard JSON
		{"application/json", true, "bare application/json"},
		{"application/json; charset=utf-8", true, "application/json with charset"},
		{"application/json;charset=utf-8", true, "application/json with charset, no space"},
		{"application/json; charset=UTF-8", true, "application/json with uppercase charset"},

		// +json structured-syntax suffix (RFC 6839)
		{"application/vnd.api+json", true, "application/vnd.api+json"},
		{"application/ld+json", true, "application/ld+json"},
		{"application/problem+json", true, "application/problem+json"},
		{"application/vnd.api+json; charset=utf-8", true, "+json with charset"},

		// Non-JSON types
		{"text/html", false, "text/html"},
		{"text/plain", false, "text/plain"},
		{"application/x-www-form-urlencoded", false, "form urlencoded"},
		{"multipart/form-data", false, "multipart form data"},
		{"application/xml", false, "application/xml"},
		{"application/octet-stream", false, "application/octet-stream"},

		// Edge cases
		{"", false, "empty string"},
		{"not-a-content-type", false, "malformed content type"},
		{"application/jsonx", false, "application/jsonx should not match"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			got := hasJSONContentType(tc.ct)
			if got != tc.want {
				t.Errorf("hasJSONContentType(%q) = %v, want %v", tc.ct, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CSRFMiddlewareJSON integration tests
//
// The same session-roundtrip pattern used by csrf_test.go:
//   1. GET through (SessionMiddleware → CSRFMiddlewareJSON → echoTokenHandler)
//      to obtain a session cookie and CSRF token.
//   2. POST with the cookie and the token (or a wrong one) and assert the
//      response code.
// ---------------------------------------------------------------------------

// makeJSONCSRFHandler wraps CSRFMiddlewareJSON with a session manager so that
// CSRFToken(r) works correctly. The inner handler echoes the CSRF token so
// tests can read it after a GET.
func makeJSONCSRFHandler() (http.Handler, *SessionManager) {
	sm := DefaultSessionManager()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(CSRFToken(r)))
	})
	// Session middleware must wrap everything so the session is available to
	// CSRFMiddlewareJSON when it calls CSRFToken(r).
	return sm.Middleware()(CSRFMiddleware()(CSRFMiddlewareJSON()(inner))), sm
}

// getTokenAndCookie performs a GET through handler and returns the CSRF token
// and the raw Cookie header to replay in subsequent requests.
func getTokenAndCookie(t *testing.T, handler http.Handler) (token, cookieHeader string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET setup: expected 200, got %d", rr.Code)
	}
	token = strings.TrimSpace(rr.Body.String())
	if token == "" {
		t.Fatal("GET setup: expected CSRF token in body, got empty string")
	}
	for _, c := range rr.Result().Cookies() {
		cookieHeader += c.Name + "=" + c.Value + "; "
	}
	return token, cookieHeader
}

func TestCSRFMiddlewareJSON_AllowsSafeMethod(t *testing.T) {
	t.Parallel()
	handler, _ := makeJSONCSRFHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rr.Code)
	}
}

func TestCSRFMiddlewareJSON_SkipsNonJSONContentType(t *testing.T) {
	t.Parallel()
	// A POST with form content-type and no CSRF token header should NOT be
	// blocked by CSRFMiddlewareJSON (form CSRF is handled by CSRFMiddleware).
	// To isolate the JSON middleware use it alone (without CSRFMiddleware).
	sm := DefaultSessionManager()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := sm.Middleware()(CSRFMiddlewareJSON()(inner))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("form POST should pass through JSON middleware; got %d", rr.Code)
	}
}

func TestCSRFMiddlewareJSON_BlocksJSONWithMissingToken(t *testing.T) {
	t.Parallel()
	handler, _ := makeJSONCSRFHandler()
	_, cookie := getTokenAndCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	// No X-CSRF-Token header.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing token, got %d", rr.Code)
	}
}

func TestCSRFMiddlewareJSON_BlocksJSONWithWrongToken(t *testing.T) {
	t.Parallel()
	handler, _ := makeJSONCSRFHandler()
	_, cookie := getTokenAndCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-CSRF-Token", "definitely-wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong token, got %d", rr.Code)
	}
}

func TestCSRFMiddlewareJSON_AllowsJSONWithCorrectToken(t *testing.T) {
	t.Parallel()
	handler, _ := makeJSONCSRFHandler()
	token, cookie := getTokenAndCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("expected request with correct token to pass, got 403")
	}
}

func TestCSRFMiddlewareJSON_AllowsVendorJSONWithCorrectToken(t *testing.T) {
	t.Parallel()
	// application/vnd.api+json must also be treated as JSON (RFC 6839 +json suffix).
	handler, _ := makeJSONCSRFHandler()
	token, cookie := getTokenAndCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("expected application/vnd.api+json with correct token to pass, got 403")
	}
}

func TestCSRFMiddlewareJSON_AllowsJSONWithCharsetAndCorrectToken(t *testing.T) {
	t.Parallel()
	// application/json; charset=utf-8 must work (mime parameters stripped correctly).
	handler, _ := makeJSONCSRFHandler()
	token, cookie := getTokenAndCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("expected 'application/json; charset=utf-8' with correct token to pass, got 403")
	}
}
