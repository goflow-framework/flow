package flow

import (
    "net"
    "net/http"
    "strings"
    "sync"

    "golang.org/x/time/rate"
)

// simple per-client rate limiter using golang.org/x/time/rate. This is
// intentionally small and in-memory: it is suitable for single-process
// deployments and for providing reasonable default protection. Users who need
// global or distributed rate limiting should replace this middleware with a
// bespoke implementation backed by a shared store (Redis, etc.).

var (
    rlMu sync.Mutex
    rl   = map[string]*rate.Limiter{}
)

func getLimiter(client string, rps int, burst int) *rate.Limiter {
    rlMu.Lock()
    defer rlMu.Unlock()
    if l, ok := rl[client]; ok {
        return l
    }
    l := rate.NewLimiter(rate.Limit(rps), burst)
    rl[client] = l
    return l
}

// RateLimitMiddleware returns a middleware that rate-limits requests per
// client IP. rps is the allowed requests per second and burst is the
// maximum burst size. A zero rps disables the limiter.
func RateLimitMiddleware(rps int, burst int) Middleware {
    if rps <= 0 {
        // no-op middleware
        return func(next http.Handler) http.Handler { return next }
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            client := clientIP(r)
            l := getLimiter(client, rps, burst)
            if !l.Allow() {
                w.Header().Set("Retry-After", "1")
                http.Error(w, "too many requests", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// WithRateLimit registers a rate limiter on the App. If rps is zero the
// limiter is disabled. Default callers should prefer WithDefaultMiddleware
// which wires a sensible default limiter.
func WithRateLimit(rps int, burst int) Option {
    return func(a *App) {
        if a == nil {
            return
        }
        a.Use(RateLimitMiddleware(rps, burst))
    }
}

// clientIP returns the request's client IP address using X-Forwarded-For or
// RemoteAddr. It prefers X-Forwarded-For's first value if present.
func clientIP(r *http.Request) string {
    if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
        parts := strings.Split(xf, ",")
        return strings.TrimSpace(parts[0])
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}
