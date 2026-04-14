package flow

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func runAppServeBM(b *testing.B, a *App, req *http.Request) {
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.ServeHTTP(w, req)
	}
}

// Benchmark using the real App middleware stack (Recovery, RequestID, Logging, Metrics)
// and the public Router adapter.  The internal params pool is always enabled
// (production default); pool-vs-nopool comparisons live in
// internal/router/router_bench_test.go where r.useParamsPool can be toggled
// directly on the per-Router field.
func BenchmarkApp_DefaultMiddleware_Integration(b *testing.B) {
	// App with default middleware and a discard logger
	app := New("bench-app", WithLogger(log.New(io.Discard, "bench: ", log.LstdFlags)), WithDefaultMiddleware())

	// Public router binds to app and adapts handlers to *Context
	r := NewRouter(app)
	r.Get("/users/:id/profile", func(c *Context) {
		// handler reads param
		_ = c.Param("id")
		c.Status(200)
	})

	app.SetRouter(r.Handler())

	req := httptest.NewRequest("GET", "/users/123/profile", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
	}
}
