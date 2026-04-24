package flow

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeHandlerWithSession() (http.Handler, *SessionManager) {
	sm := DefaultSessionManager()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// respond with the token so tests can read it
		w.Write([]byte(CSRFToken(r)))
	})
	return sm.Middleware()(CSRFMiddleware()(h)), sm
}

func TestGetSetsCSRFToken(t *testing.T) {
	handler, sm := makeHandlerWithSession()
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	token := strings.TrimSpace(rr.Body.String())
	if token == "" {
		t.Fatalf("expected token in response body")
	}

	// cookie should be set with sm.CookieName
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == sm.CookieName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session cookie to be set")
	}
}

func TestPostWithValidTokenPasses(t *testing.T) {
	handler, _ := makeHandlerWithSession()
	// GET to obtain cookie + token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)
	token := strings.TrimSpace(getRR.Body.String())
	if token == "" {
		t.Fatalf("no token from GET")
	}

	// build Cookie header from returned cookies
	var cookieHeader string
	for _, c := range getRR.Result().Cookies() {
		cookieHeader += c.Name + "=" + c.Value + "; "
	}

	postReq := httptest.NewRequest("POST", "/", nil)
	postReq.Header.Set("Cookie", cookieHeader)
	postReq.Header.Set("X-CSRF-Token", token)
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)
	if postRR.Code == http.StatusForbidden {
		t.Fatalf("expected POST with valid token to pass; got forbidden")
	}
}

func TestPostWithInvalidTokenRejected(t *testing.T) {
	handler, _ := makeHandlerWithSession()
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	var cookieHeader string
	for _, c := range getRR.Result().Cookies() {
		cookieHeader += c.Name + "=" + c.Value + "; "
	}

	postReq := httptest.NewRequest("POST", "/", nil)
	postReq.Header.Set("Cookie", cookieHeader)
	postReq.Header.Set("X-CSRF-Token", "badtoken")
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for invalid token, got %d", postRR.Code)
	}
}

// errReader is an io.Reader that always returns an error, used to simulate
// crypto/rand failures in tests.
type errReader struct{ msg string }

func (e errReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("%s", e.msg)
}

// TestCSRFMiddleware_RandFailureReturns500 verifies that when crypto/rand is
// unavailable the middleware serves 500 instead of issuing a predictable token.
func TestCSRFMiddleware_RandFailureReturns500(t *testing.T) {
	// Swap rand reader for one that always fails, restore afterwards.
	orig := csrfRandReader
	csrfRandReader = errReader{msg: "entropy pool exhausted"}
	defer func() { csrfRandReader = orig }()

	sm := DefaultSessionManager().WithInsecureCookie()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := sm.Middleware()(CSRFMiddleware()(h))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on rand failure, got %d", rr.Code)
	}
}

// TestCSRFMiddleware_RandFailureDoesNotIssueEmptyToken verifies that on rand
// failure the body is not an empty CSRF token (i.e. no silent degradation).
func TestCSRFMiddleware_RandFailureDoesNotIssueEmptyToken(t *testing.T) {
	orig := csrfRandReader
	csrfRandReader = errReader{msg: "entropy pool exhausted"}
	defer func() { csrfRandReader = orig }()

	sm := DefaultSessionManager().WithInsecureCookie()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If we reach here a token was issued — that is wrong.
		w.Write([]byte(CSRFToken(r)))
	})
	handler := sm.Middleware()(CSRFMiddleware()(h))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Must not reach the inner handler (500 returned above).
	if rr.Code == http.StatusOK {
		t.Fatalf("handler should not have been called on rand failure")
	}
	if strings.TrimSpace(rr.Body.String()) == "" && rr.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected response: code=%d body=%q", rr.Code, rr.Body.String())
	}
}

// TestGenerateCSRFToken_ReturnsErrorOnBadReader unit-tests generateCSRFToken
// directly with a failing reader.
func TestGenerateCSRFToken_ReturnsErrorOnBadReader(t *testing.T) {
	orig := csrfRandReader
	csrfRandReader = errReader{msg: "no entropy"}
	defer func() { csrfRandReader = orig }()

	tok, err := generateCSRFToken()
	if err == nil {
		t.Fatalf("expected error from generateCSRFToken, got nil (token=%q)", tok)
	}
	if tok != "" {
		t.Fatalf("expected empty token on error, got %q", tok)
	}
}

// TestGenerateCSRFToken_SuccessReturnsNonEmpty verifies the happy path.
func TestGenerateCSRFToken_SuccessReturnsNonEmpty(t *testing.T) {
	tok, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	// base64url-encoded 32 bytes = 43 chars (no padding)
	if len(tok) != 43 {
		t.Fatalf("expected token length 43, got %d", len(tok))
	}
}

// --- nil-session guard tests -----------------------------------------------

// TestCSRFMiddleware_NoSession_GET_Returns500 verifies that CSRFMiddleware
// returns 500 with a diagnostic message on a safe method when no session
// middleware has been registered.
func TestCSRFMiddleware_NoSession_GET_Returns500(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called when session is absent")
	})
	// CSRFMiddleware registered WITHOUT wrapping in SessionManager.Middleware()
	handler := CSRFMiddleware()(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when session middleware absent, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session middleware is not configured") {
		t.Fatalf("expected diagnostic message in body, got: %q", rr.Body.String())
	}
}

// TestCSRFMiddleware_NoSession_POST_Returns500 verifies the same guard fires on
// an unsafe method so the developer gets 500 (not a mysterious 403).
func TestCSRFMiddleware_NoSession_POST_Returns500(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called when session is absent")
	})
	handler := CSRFMiddleware()(h)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "any-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when session middleware absent, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session middleware is not configured") {
		t.Fatalf("expected diagnostic message in body, got: %q", rr.Body.String())
	}
}

// TestCSRFMiddlewareJSON_NoSession_POST_Returns500 verifies that
// CSRFMiddlewareJSON returns 500 with a diagnostic message when the session
// middleware is absent on an unsafe JSON request.
func TestCSRFMiddlewareJSON_NoSession_POST_Returns500(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler must not be called when session is absent")
	})
	handler := CSRFMiddlewareJSON()(h)

	req := httptest.NewRequest(http.MethodPost, "/api/resource", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "any-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when session middleware absent, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session middleware is not configured") {
		t.Fatalf("expected diagnostic message in body, got: %q", rr.Body.String())
	}
}

// TestCSRFMiddlewareJSON_NoSession_GET_PassesThrough verifies that safe methods
// are not affected by the nil-session guard in CSRFMiddlewareJSON (no
// validation needed on GET).
func TestCSRFMiddlewareJSON_NoSession_GET_PassesThrough(t *testing.T) {
	reached := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFMiddlewareJSON()(h)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !reached {
		t.Fatal("expected inner handler to be reached for GET with CSRFMiddlewareJSON")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
