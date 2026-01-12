package flow

import (
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
