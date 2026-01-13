package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"log"
)

// simple handler used by benchmarks
func benchHandler(w http.ResponseWriter, r *http.Request) {}

func runServeBM(b *testing.B, r *Router, req *http.Request) {
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMatchRoute_NoParams(b *testing.B) {
	types := []struct {
		name    string
		usePool bool
	}{
		{"pool", true},
		{"nopool", false},
	}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			UseParamsPool = tt.usePool
			r := New()
			r.Get("/ping", benchHandler)
			req := httptest.NewRequest("GET", "/ping", nil)
			runServeBM(b, r, req)
		})
	}
}

func BenchmarkMatchRoute_WithParams(b *testing.B) {
	types := []struct {
		name    string
		usePool bool
	}{
		{"pool", true},
		{"nopool", false},
	}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			UseParamsPool = tt.usePool
			r := New()
			r.Get("/users/:id/profile", benchHandler)
			req := httptest.NewRequest("GET", "/users/123/profile", nil)
			runServeBM(b, r, req)
		})
	}
}

// A slightly heavier benchmark with multiple routes to force iteration.
func BenchmarkMatchRoute_ManyRoutes(b *testing.B) {
	types := []struct {
		name    string
		usePool bool
	}{
		{"pool", true},
		{"nopool", false},
	}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			UseParamsPool = tt.usePool
			r := New()
			// register many routes
			for i := 0; i < 50; i++ {
				p := "/static/route/" + strconv.Itoa(i)
				r.Get(p, benchHandler)
			}
			// add param route near the end
			r.Get("/items/:itemid/detail", benchHandler)

			req := httptest.NewRequest("GET", "/items/42/detail", nil)
			runServeBM(b, r, req)
		})
	}
}

// Integration benchmark: multiple middleware that each read a route parameter
func BenchmarkIntegration_MiddlewareAndHandlerReadingParams(b *testing.B) {
	types := []struct {
		name    string
		usePool bool
	}{
		{"pool", true},
		{"nopool", false},
	}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			UseParamsPool = tt.usePool
			r := New()

			// middleware that reads the "id" param from context
			mwRead := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = Param(r, "id")
					next.ServeHTTP(w, r)
				})
			}

			// register route with several middleware layers
			r.GetWith("/users/:id/profile", func(w http.ResponseWriter, r *http.Request) {
				// final handler also reads the param
				_ = Param(r, "id")
				w.WriteHeader(200)
			}, mwRead, mwRead, mwRead)

			req := httptest.NewRequest("GET", "/users/123/profile", nil)
			runServeBM(b, r, req)
		})
	}
}

// Integration benchmark: simulate default middleware stack (RequestID, Logging, Metrics)
func BenchmarkIntegration_DefaultMiddlewareStack(b *testing.B) {
	types := []struct {
		name    string
		usePool bool
	}{{"pool", true}, {"nopool", false}}
	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			UseParamsPool = tt.usePool
			r := New()

			// RequestID middleware: ensure X-Request-ID exists and is returned in response
			reqIDMW := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					id := r.Header.Get("X-Request-ID")
					if id == "" {
						// generate a UUID string
						id = uuid.New().String()
						r.Header.Set("X-Request-ID", id)
					}
					w.Header().Set("X-Request-ID", id)
					next.ServeHTTP(w, r)
				})
			}

			// Logging middleware: log start and end (to io.Discard) to simulate real logger cost
			logger := log.New(io.Discard, "bench: ", log.LstdFlags)
			loggingMW := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start := time.Now()
					logger.Printf("request start: %s %s", r.Method, r.URL.Path)
					next.ServeHTTP(w, r)
					logger.Printf("request complete: %s %s in %s", r.Method, r.URL.Path, time.Since(start))
				})
			}

			// Metrics middleware: measure time and set X-Response-Time header
			metricsMW := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start := time.Now()
					next.ServeHTTP(w, r)
					w.Header().Set("X-Response-Time", strconv.FormatInt(time.Since(start).Milliseconds(), 10)+"ms")
				})
			}

			// final handler reads the param
			h := func(w http.ResponseWriter, r *http.Request) {
				_ = Param(r, "id")
				w.WriteHeader(200)
			}

			// Register route with middleware in the conventional order: RequestID, Logging, Metrics
			r.GetWith("/users/:id/profile", h, reqIDMW, loggingMW, metricsMW)

			req := httptest.NewRequest("GET", "/users/123/profile", nil)
			runServeBM(b, r, req)
		})
	}
}
