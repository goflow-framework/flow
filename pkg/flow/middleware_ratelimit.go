package flow

// Rate-limiting middleware with trusted-proxy support.
//
// Security model
// --------------
// X-Forwarded-For (XFF) is a client-controlled header: any HTTP client can
// set it to an arbitrary value. Trusting XFF unconditionally lets attackers
// trivially bypass per-IP rate limiting by cycling the header value.
//
// The correct approach is to only honour XFF when the TCP peer (RemoteAddr)
// is a known, trusted proxy. TrustedProxies holds a list of CIDRs that
// represent the proxy tier; the middleware walks the XFF list right-to-left
// and stops at the first IP that is NOT in the trusted set — that is the real
// client IP.
//
// If TrustedProxies is empty (the default) RemoteAddr is always used and XFF
// is completely ignored, making the default configuration safe out-of-the-box.
//
// Memory safety
// -------------
// The previous implementation used a process-global map that was never
// evicted. Every unique client IP received a permanent entry, enabling a
// trivial memory-exhaustion attack via IP cycling. Each call to
// RateLimitMiddleware now creates its own per-closure map. A background
// cleanup goroutine evicts limiters that have been idle longer than
// limiterTTL (5 minutes), bounding memory to O(unique-active-clients).

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DefaultRateLimitRPS and DefaultRateLimitBurst are conservative defaults used
// by WithDefaultMiddleware. They are intentionally modest to avoid surprising
// throttling for new apps while still providing baseline protection.
const (
	DefaultRateLimitRPS   = 10
	DefaultRateLimitBurst = 20
)

// limiterTTL is how long a per-IP limiter can be idle before it is evicted
// from the per-middleware map. Eviction runs every limiterTTL/2.
const limiterTTL = 5 * time.Minute

// RateLimitOptions configures RateLimitMiddleware.
type RateLimitOptions struct {
	// RPS is the sustained request rate per second per client IP.
	// A zero or negative value disables the limiter (no-op middleware).
	RPS int

	// Burst is the maximum burst size above RPS.
	Burst int

	// TrustedProxies is the list of CIDR ranges whose X-Forwarded-For header
	// is trusted. When empty (the default), XFF is always ignored and the
	// TCP RemoteAddr is used as the client IP. This is the safe default.
	//
	// Example for a typical single-proxy deployment:
	//   TrustedProxies: flow.MustParseCIDRs([]string{"10.0.0.0/8", "172.16.0.0/12"})
	TrustedProxies []*net.IPNet
}

// MustParseCIDRs parses a slice of CIDR strings and panics on the first
// invalid entry. Intended for use in package-level var blocks or test setup.
//
//	var proxies = flow.MustParseCIDRs([]string{"10.0.0.0/8", "172.16.0.0/12"})
func MustParseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			panic("flow: invalid CIDR " + c + ": " + err.Error())
		}
		out = append(out, ipnet)
	}
	return out
}

// ParseCIDRs parses a slice of CIDR strings and returns the results together
// with the first parse error (if any). Unlike MustParseCIDRs it does not
// panic and is suitable for use in config-loading paths.
func ParseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}
		out = append(out, ipnet)
	}
	return out, nil
}

// isTrustedProxy reports whether ip is within one of the provided trusted
// CIDR ranges.
func isTrustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	for _, cidr := range trusted {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIPFromRequest resolves the real client IP for a request given a list
// of trusted proxy CIDRs.
//
// When trustedProxies is empty, RemoteAddr is always returned (safe default).
//
// When trustedProxies is non-empty the function inspects X-Forwarded-For.
// XFF is a comma-separated list of IPs appended left-to-right as the request
// passes through each proxy: "client, proxy1, proxy2". We walk from the
// rightmost entry and peel off entries that belong to a trusted proxy. The
// leftmost entry that is NOT a trusted proxy is the real client IP.
//
// If XFF is absent or all entries are trusted proxies, RemoteAddr is returned.
func clientIPFromRequest(r *http.Request, trustedProxies []*net.IPNet) string {
	// Fast path: no trusted proxies configured → always use RemoteAddr.
	if len(trustedProxies) == 0 {
		return remoteAddrIP(r)
	}

	// Check whether the direct peer (RemoteAddr) is itself a trusted proxy.
	// If not, XFF cannot be trusted at all.
	peerIP := net.ParseIP(remoteAddrIP(r))
	if peerIP == nil || !isTrustedProxy(peerIP, trustedProxies) {
		// The TCP peer is not a trusted proxy; ignore XFF entirely.
		return remoteAddrIP(r)
	}

	// The peer is a trusted proxy. Walk XFF right-to-left to find the
	// leftmost IP that is not a trusted proxy.
	xf := r.Header.Get("X-Forwarded-For")
	if xf == "" {
		return remoteAddrIP(r)
	}

	parts := strings.Split(xf, ",")
	// Walk from right to left, skipping trusted proxies.
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		ip := net.ParseIP(candidate)
		if ip == nil {
			// Unparseable entry — stop here to prevent spoofing via
			// malformed entries further left in the list.
			return remoteAddrIP(r)
		}
		if !isTrustedProxy(ip, trustedProxies) {
			// First non-trusted entry from the right is the real client.
			return ip.String()
		}
	}

	// All XFF entries were trusted proxies; fall back to RemoteAddr.
	return remoteAddrIP(r)
}

// remoteAddrIP extracts the IP portion of r.RemoteAddr, stripping the port.
func remoteAddrIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr has no port (e.g. in some test scenarios).
		return r.RemoteAddr
	}
	return host
}

// limiterEntry pairs a rate.Limiter with the last time it was accessed, used
// for TTL eviction.
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitMiddleware returns a Middleware that rate-limits requests per client
// IP. rps is the allowed requests per second and burst is the maximum burst
// size. A zero or negative rps returns a no-op middleware.
//
// This variant uses no trusted proxies (safe default): X-Forwarded-For is
// ignored and the TCP RemoteAddr is always used as the client identifier.
// Use RateLimitMiddlewareWithOptions when you need to honour XFF from a known
// proxy tier.
//
// Each call to RateLimitMiddleware creates its own per-closure limiter map and
// a background eviction goroutine, eliminating the global-map state pollution
// of the previous implementation.
func RateLimitMiddleware(rps int, burst int) Middleware {
	return RateLimitMiddlewareWithOptions(RateLimitOptions{RPS: rps, Burst: burst})
}

// RateLimitMiddlewareWithOptions is the full-featured variant that accepts a
// RateLimitOptions struct. Use this when deploying behind a reverse proxy and
// you need XFF to be honoured for accurate per-client rate limiting.
func RateLimitMiddlewareWithOptions(opts RateLimitOptions) Middleware {
	if opts.RPS <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	mu := sync.Mutex{}
	limiters := make(map[string]*limiterEntry)

	// Background eviction: remove limiters idle for longer than limiterTTL.
	go func() {
		ticker := time.NewTicker(limiterTTL / 2)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for ip, e := range limiters {
				if now.Sub(e.lastSeen) > limiterTTL {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := clientIPFromRequest(r, opts.TrustedProxies)

			mu.Lock()
			e, ok := limiters[client]
			if !ok {
				e = &limiterEntry{
					limiter: rate.NewLimiter(rate.Limit(opts.RPS), opts.Burst),
				}
				limiters[client] = e
			}
			e.lastSeen = time.Now()
			l := e.limiter
			mu.Unlock()

			if !l.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithRateLimit registers a rate limiter on the App with default options
// (no trusted proxies — safe for direct-to-internet deployments).
// If rps is zero the limiter is disabled.
func WithRateLimit(rps int, burst int) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(RateLimitMiddleware(rps, burst))
	}
}

// WithRateLimitOptions registers a rate limiter on the App with full options,
// including trusted proxy CIDRs for correct XFF handling.
//
//	app := flow.New("myapp",
//	    flow.WithRateLimitOptions(flow.RateLimitOptions{
//	        RPS:   50,
//	        Burst: 100,
//	        TrustedProxies: flow.MustParseCIDRs([]string{"10.0.0.0/8"}),
//	    }),
//	)
func WithRateLimitOptions(opts RateLimitOptions) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(RateLimitMiddlewareWithOptions(opts))
	}
}
