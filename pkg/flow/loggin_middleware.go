package flow

import (
	"net/http"
	"time"
)

// LoggingMiddlewareWithRedaction is a drop-in improved logging middleware that
// prefers StructuredLogger when available and redacts fields before logging.
func LoggingMiddlewareWithRedaction(logger Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
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
