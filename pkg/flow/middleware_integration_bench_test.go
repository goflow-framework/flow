package flow

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	internalrouter "github.com/dministrator/flow/internal/router"
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
// and the public Router adapter. Compares behavior with and without params pooling.
func BenchmarkApp_DefaultMiddleware_Integration(b *testing.B) {
	types := []struct{ name string; usePool bool }{{"pool", true}, {"nopool", false}}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			// toggle internal router pooling
			internalrouter.UseParamsPool = tt.usePool

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
			runAppServeBM(b, app, req)
		})
	}
}
