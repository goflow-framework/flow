package flow

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestIDMiddleware_App(t *testing.T) {
	app := New("test-app", WithRequestID(""))

	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the middleware should have injected X-Request-ID into the request header
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Fatalf("expected X-Request-ID in request header")
		}
		// also ensure response header is set
		wid := w.Header().Get("X-Request-ID")
		// middleware typically sets response header before handler runs; accept either
		w.WriteHeader(200)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	// response should also include X-Request-ID
	if got := rr.Result().Header.Get("X-Request-ID"); got == "" {
		t.Fatalf("expected X-Request-ID in response header")
	}
}

func TestTimeoutMiddleware_CancelsHandler(t *testing.T) {
	// short timeout
	app := New("test-timeout", WithTimeout(20*time.Millisecond))

	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			// would have completed if not canceled
			w.WriteHeader(200)
		case <-r.Context().Done():
			// handler should notice cancellation
			w.WriteHeader(499)
		}
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != 499 {
		t.Fatalf("expected handler to observe cancellation and return 499, got %d", rr.Code)
	}
}
