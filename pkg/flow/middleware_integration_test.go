package flow

// middleware_integration_test.go exercises the full middleware stack as it
// is wired by WithDefaultMiddleware and the individual With* options, using
// a real App instance served via httptest.  These tests assert observable
// HTTP behaviour — headers, status codes, body content — not implementation
// details.

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestApp returns an App with a discard logger to keep test output clean.
func newTestApp(t *testing.T, opts ...Option) *App {
	t.Helper()
	return New("test-app", append([]Option{WithLogger(log.New(io.Discard, "", 0))}, opts...)...)
}

// ─── RequestID middleware ──────────────────────────────────────────────────

func TestRequestIDMiddleware_SetsHeaderWhenAbsent(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, WithRequestID(""))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("expected X-Request-ID response header to be set")
	}
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	t.Parallel()
	const clientID = "my-trace-id-abc"

	app := newTestApp(t, WithRequestID(""))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// middleware must propagate the client-supplied ID into the request
		got := r.Header.Get("X-Request-ID")
		if got != clientID {
			t.Errorf("handler: X-Request-ID in request = %q, want %q", got, clientID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", clientID)
	app.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != clientID {
		t.Errorf("response X-Request-ID = %q, want %q", got, clientID)
	}
}

func TestRequestIDMiddleware_UniquePerRequest(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, WithRequestID(""))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		app.Handler().ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		if id == "" {
			t.Fatal("got empty X-Request-ID")
		}
		if _, dup := ids[id]; dup {
			t.Fatalf("duplicate X-Request-ID %q after %d requests", id, i)
		}
		ids[id] = struct{}{}
	}
}

// ─── Recovery middleware ───────────────────────────────────────────────────

func TestRecoveryMiddleware_Returns500OnPanic(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(Recovery(app.logger))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("intentional test panic")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on panic, got %d", rec.Code)
	}
}

func TestRecoveryMiddleware_ContinuesServingAfterPanic(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(Recovery(app.logger))

	calls := 0
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			panic("first request panics")
		}
		w.WriteHeader(http.StatusOK)
	}))

	h := app.Handler()

	// first request — panics
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on first (panicking) request, got %d", rec1.Code)
	}

	// second request — must succeed; server is still alive
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on second request after recovered panic, got %d", rec2.Code)
	}
}

// ─── Metrics middleware ────────────────────────────────────────────────────

func TestMetricsMiddleware_SetsXResponseTimeHeader(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(MetricsMiddleware())
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("X-Response-Time"); got == "" {
		t.Fatal("expected X-Response-Time header to be set by MetricsMiddleware")
	}
}

func TestMetricsMiddleware_HeaderContainsMs(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(MetricsMiddleware())
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	val := rec.Header().Get("X-Response-Time")
	if !strings.HasSuffix(val, "ms") {
		t.Fatalf("X-Response-Time = %q, want value ending in 'ms'", val)
	}
}

// ─── Timeout middleware ────────────────────────────────────────────────────

func TestTimeoutMiddleware_CancelsSlowHandler(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, WithTimeout(20*time.Millisecond))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 (handler observed timeout), got %d", rec.Code)
	}
}

func TestTimeoutMiddleware_FastHandlerCompletes(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, WithTimeout(200*time.Millisecond))
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fast", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for fast handler under timeout, got %d", rec.Code)
	}
}

// ─── Full default stack integration ───────────────────────────────────────

// TestDefaultMiddleware_HeadersPresent asserts that a request processed by
// WithDefaultMiddleware gets all expected response headers set.
// CSRF and RateLimit are skipped here (they need session/state setup), so we
// build the subset that is unconditionally safe to assert.
func TestDefaultMiddleware_RequestIDAndMetricsPresent(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(Recovery(app.logger))
	app.Use(RequestIDMiddleware(""))
	app.Use(MetricsMiddleware())
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got == "" {
		t.Error("expected X-Request-ID header")
	}
	if got := rec.Header().Get("X-Response-Time"); got == "" {
		t.Error("expected X-Response-Time header")
	}
}

// TestDefaultMiddleware_RecoveryWrapsOtherMiddleware verifies that Recovery
// is the outermost layer: a panic inside a downstream middleware is caught.
func TestDefaultMiddleware_RecoveryWrapsOtherMiddleware(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(Recovery(app.logger))
	// inner middleware that panics
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("inner middleware panic")
		})
	})
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when inner middleware panics, got %d", rec.Code)
	}
}

// ─── Middleware stack ordering ─────────────────────────────────────────────

// TestMiddleware_OrderIsLIFO confirms that middleware registered first wraps
// outermost (LIFO execution order through the chain).
func TestMiddleware_OrderIsLIFO(t *testing.T) {
	t.Parallel()
	var order []string
	mu := sync.Mutex{}
	record := func(label string) {
		mu.Lock()
		order = append(order, label)
		mu.Unlock()
	}

	app := newTestApp(t)
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record("first-before")
			next.ServeHTTP(w, r)
			record("first-after")
		})
	})
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record("second-before")
			next.ServeHTTP(w, r)
			record("second-after")
		})
	})
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		record("handler")
		w.WriteHeader(http.StatusOK)
	}))

	app.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"first-before", "second-before", "handler", "second-after", "first-after"}
	if len(order) != len(want) {
		t.Fatalf("execution order length = %d, want %d; got %v", len(order), len(want), order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Errorf("order[%d] = %q, want %q", i, order[i], w)
		}
	}
}

// ─── Concurrency safety ───────────────────────────────────────────────────

// TestMiddlewareStack_ConcurrentRequests exercises the middleware stack from
// multiple goroutines simultaneously.  Run with -race to detect data races.
func TestMiddlewareStack_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	app.Use(Recovery(app.logger))
	app.Use(RequestIDMiddleware(""))
	app.Use(MetricsMiddleware())
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h := app.Handler()
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("concurrent request: expected 200, got %d", rec.Code)
				}
			}
		}()
	}
	wg.Wait()
}

// ─── Table-driven: middleware option smoke tests ───────────────────────────

func TestMiddlewareOptions_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		path       string
		opts       []Option
		handler    http.HandlerFunc
		wantStatus int
		wantHeader string // header key that must be non-empty, or ""
	}{
		{
			name:   "WithRequestID sets header",
			method: http.MethodGet,
			path:   "/",
			opts:   []Option{WithRequestID("")},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantStatus: http.StatusOK,
			wantHeader: "X-Request-ID",
		},
		{
			name:   "WithMetrics sets X-Response-Time",
			method: http.MethodGet,
			path:   "/metrics",
			opts:   []Option{WithMetrics()},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantStatus: http.StatusNoContent,
			wantHeader: "X-Response-Time",
		},
		{
			name:   "WithBodyLimit allows small body",
			method: http.MethodPost,
			path:   "/upload",
			opts:   []Option{WithBodyLimit(1024)},
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			},
			wantStatus: http.StatusOK,
			wantHeader: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newTestApp(t, tc.opts...)
			app.SetRouter(tc.handler)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			app.Handler().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantHeader != "" {
				if got := rec.Header().Get(tc.wantHeader); got == "" {
					t.Fatalf("expected response header %q to be set", tc.wantHeader)
				}
			}
		})
	}
}
