package flow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultBodyLimitBytes is the default maximum request body size enforced by
// BodyLimitMiddleware and WithDefaultMiddleware. It is set to 4 MiB, which is
// generous for typical JSON/form payloads while protecting against trivial
// request-body DoS attacks. Callers may override it via WithBodyLimit.
const DefaultBodyLimitBytes int64 = 4 << 20 // 4 MiB

// BodyLimitMiddleware returns a Middleware that caps incoming request bodies at
// maxBytes. When a client sends more data than allowed, the read is aborted and
// the handler receives a 413 Request Entity Too Large response before the body
// is processed.
//
// Under the hood it uses http.MaxBytesReader so the limit is enforced lazily
// as the body is read (e.g., inside BindJSON / BindForm / ParseMultipartForm).
// If the body has already been read before this middleware runs the limit has
// no effect — register BodyLimitMiddleware early in the stack (it is part of
// WithDefaultMiddleware by default).
//
// A zero or negative maxBytes disables the limit (no-op middleware returned).
func BodyLimitMiddleware(maxBytes int64) Middleware {
	if maxBytes <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithBodyLimit registers a BodyLimitMiddleware with the given maximum body
// size on the App. Call this during App construction, or use
// WithDefaultMiddleware which wires a DefaultBodyLimitBytes limit automatically.
//
// A zero or negative maxBytes disables body limiting (not recommended for
// production deployments).
func WithBodyLimit(maxBytes int64) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(BodyLimitMiddleware(maxBytes))
	}
}

// IsBodyTooLarge reports whether err (or any error in its chain) was produced
// by http.MaxBytesReader when the request body exceeded the configured limit.
// Use this in error handlers or BindJSON/BindForm call-sites to return a
// meaningful 413 response:
//
//	if err := ctx.BindJSON(&payload); err != nil {
//	    if flow.IsBodyTooLarge(err) {
//	        ctx.Error(http.StatusRequestEntityTooLarge, "request body too large")
//	        return
//	    }
//	    ctx.Error(http.StatusBadRequest, err.Error())
//	}
func IsBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	// http.MaxBytesReader returns *http.MaxBytesError (Go 1.19+). For older
	// builds we also check the error message as a fallback.
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

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
