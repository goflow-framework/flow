package flow

import (
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"

    "golang.org/x/time/rate"
)

// ensure clean global limiter state between tests
func resetLimiters() {
    rlMu.Lock()
    rl = map[string]*rate.Limiter{}
    rlMu.Unlock()
}

func TestRateLimit_AllowsRequests(t *testing.T) {
    resetLimiters()

    var called int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&called, 1)
        w.WriteHeader(http.StatusOK)
    })

    // generous limits so all requests should pass
    mw := RateLimitMiddleware(100, 10)(next)

    attempts := 10
    for i := 0; i < attempts; i++ {
        req := httptest.NewRequest("GET", "http://example.test/", nil)
        rr := httptest.NewRecorder()
        mw.ServeHTTP(rr, req)
        if rr.Code != http.StatusOK {
            t.Fatalf("expected status 200, got %d on attempt %d", rr.Code, i)
        }
    }

    if atomic.LoadInt32(&called) != int32(attempts) {
        t.Fatalf("expected handler called %d times, got %d", attempts, called)
    }
}

func TestRateLimit_RejectsWhenExceeded(t *testing.T) {
    resetLimiters()

    var called int32
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&called, 1)
        w.WriteHeader(http.StatusOK)
    })

    // tight limits: 1 rps, burst 1 — multiple immediate requests should be rejected
    mw := RateLimitMiddleware(1, 1)(next)

    attempts := 5
    var success int
    var rejected int
    for i := 0; i < attempts; i++ {
        req := httptest.NewRequest("GET", "http://example.test/", nil)
        rr := httptest.NewRecorder()
        mw.ServeHTTP(rr, req)
        if rr.Code == http.StatusOK {
            success++
        } else if rr.Code == http.StatusTooManyRequests {
            rejected++
        } else {
            t.Fatalf("unexpected status %d", rr.Code)
        }
    }

    if success == attempts {
        t.Fatalf("expected some requests to be rejected, but all %d succeeded", attempts)
    }
    if rejected == 0 {
        t.Fatalf("expected at least one rejected request, got 0")
    }

    if atomic.LoadInt32(&called) != int32(success) {
        t.Fatalf("expected handler called %d times, got %d", success, called)
    }
}
