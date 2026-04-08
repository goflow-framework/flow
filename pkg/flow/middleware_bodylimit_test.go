package flow

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okHandler is a trivial handler that reads the entire request body and
// responds 200. Used to verify that normal-sized bodies pass through.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "read:%d", buf.Len())
})

func TestBodyLimitMiddleware_BelowLimit(t *testing.T) {
	t.Parallel()

	mw := BodyLimitMiddleware(100)
	handler := mw(okHandler)

	body := strings.NewReader(strings.Repeat("a", 50))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "read:50") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestBodyLimitMiddleware_AtLimit(t *testing.T) {
	t.Parallel()

	const limit = 10
	mw := BodyLimitMiddleware(limit)
	handler := mw(okHandler)

	body := strings.NewReader(strings.Repeat("b", limit))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Exactly at limit: should succeed.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 at exact limit, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_ExceedsLimit(t *testing.T) {
	t.Parallel()

	const limit int64 = 10
	mw := BodyLimitMiddleware(limit)

	// handler that tries to read the entire body — MaxBytesReader will error
	// on the read itself; the actual HTTP response code depends on whether the
	// handler checks the error. We verify that IsBodyTooLarge detects it.
	var readErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, readErr = buf.ReadFrom(r.Body)
		if readErr != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(strings.Repeat("x", int(limit)+1))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
	if !IsBodyTooLarge(readErr) {
		t.Fatalf("IsBodyTooLarge should be true, got err=%v", readErr)
	}
}

func TestBodyLimitMiddleware_ZeroDisablesLimit(t *testing.T) {
	t.Parallel()

	mw := BodyLimitMiddleware(0) // no-op
	handler := mw(okHandler)

	// Send 1 MiB — should pass through without error.
	body := strings.NewReader(strings.Repeat("z", 1<<20))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with limit disabled, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_NilBody(t *testing.T) {
	t.Parallel()

	mw := BodyLimitMiddleware(100)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET requests have no body (nil).
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for nil body request, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_NegativeDisablesLimit(t *testing.T) {
	t.Parallel()

	mw := BodyLimitMiddleware(-1) // also no-op
	handler := mw(okHandler)

	body := strings.NewReader(strings.Repeat("n", 200))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with negative limit (disabled), got %d", rec.Code)
	}
}

func TestWithBodyLimit_Option(t *testing.T) {
	t.Parallel()

	app := New("test", WithBodyLimit(5))

	var readErr error
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, readErr = buf.ReadFrom(r.Body)
		if readErr != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(strings.Repeat("o", 10)) // 10 bytes > 5 byte limit
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 via WithBodyLimit option, got %d", rec.Code)
	}
	if !IsBodyTooLarge(readErr) {
		t.Fatalf("expected IsBodyTooLarge=true, got err=%v", readErr)
	}
}

func TestIsBodyTooLarge_NilErr(t *testing.T) {
	if IsBodyTooLarge(nil) {
		t.Fatal("IsBodyTooLarge(nil) should be false")
	}
}

func TestIsBodyTooLarge_UnrelatedErr(t *testing.T) {
	if IsBodyTooLarge(fmt.Errorf("some other error")) {
		t.Fatal("IsBodyTooLarge should be false for unrelated errors")
	}
}

func TestDefaultBodyLimitBytes(t *testing.T) {
	const expected int64 = 4 << 20 // 4 MiB
	if DefaultBodyLimitBytes != expected {
		t.Fatalf("DefaultBodyLimitBytes = %d, want %d", DefaultBodyLimitBytes, expected)
	}
}
