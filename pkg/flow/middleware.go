package flow

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// reqIDCounter is an atomically-incremented counter used by fastRequestID.
// It's intentionally simple and non-cryptographic — suitable for logs/tracing
// but not for security-sensitive identifiers.
var reqIDCounter uint64

// fastRequestID returns a short, fast request id using the current timestamp
// (base36) plus an atomically-incremented counter (base36). This avoids
// per-request crypto/syscall work from uuid generation while keeping ids
// reasonably unique and human-readable.
func fastRequestID() string {
	ts := strconv.FormatInt(time.Now().UnixNano(), 36)
	cnt := atomic.AddUint64(&reqIDCounter, 1)
	return ts + "-" + strconv.FormatUint(cnt, 36)
}

// LoggingMiddleware logs basic request info using the provided Logger.
func LoggingMiddleware(logger Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// Prefer structured logging when available. Build a small fields map
			// and redact sensitive values before emitting.
			fields := map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			}
			if sl, ok := logger.(StructuredLogger); ok {
				sl.Log("info", "request start", RedactMap(fields))
			} else {
				logger.Printf("request start: %s %s", r.Method, r.URL.Path)
			}
			next.ServeHTTP(w, r)
			elapsed := time.Since(start)
			fields["elapsed"] = elapsed.String()
			if sl, ok := logger.(StructuredLogger); ok {
				sl.Log("info", "request complete", RedactMap(fields))
			} else {
				logger.Printf("request complete: %s %s in %s", r.Method, r.URL.Path, elapsed)
			}
		})
	}
}

// RequestIDMiddleware sets a request id header for tracing.
func RequestIDMiddleware(headerName string) Middleware {
	if headerName == "" {
		headerName = "X-Request-ID"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(headerName)
			if id == "" {
				id = fastRequestID()
				r.Header.Set(headerName, id)
			}
			w.Header().Set(headerName, id)
			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware sets a per-request timeout; when the timeout elapses
// the request context will be cancelled. The handler should respect ctx.Done().
func TimeoutMiddleware(d time.Duration) Middleware {
	if d <= 0 {
		d = 0
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if d <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MetricsMiddleware records simple timing metrics and sets an X-Response-Time header.
func MetricsMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			elapsed := time.Since(start)
			w.Header().Set("X-Response-Time", fmt.Sprintf("%dms", elapsed.Milliseconds()))
		})
	}
}

// LoggingMiddlewareWithRedaction prefers StructuredLogger when available and redacts fields before logging.
func LoggingMiddlewareWithRedaction(logger Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// small set of fields to emit
			fields := map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			}
			if sl, ok := logger.(StructuredLogger); ok {
				sl.Log("info", "request start", RedactMap(fields))
			} else {
				logger.Printf("request start: %s %s", r.Method, r.URL.Path)
			}

			next.ServeHTTP(w, r)
			elapsed := time.Since(start)
			fields["elapsed"] = elapsed.String()
			if sl, ok := logger.(StructuredLogger); ok {
				sl.Log("info", "request complete", RedactMap(fields))
			} else {
				logger.Printf("request complete: %s %s in %s", r.Method, r.URL.Path, elapsed)
			}
		})
	}
}
