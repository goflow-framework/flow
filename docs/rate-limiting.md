# Rate Limiting & Trusted Proxies

Flow ships a built-in, per-IP token-bucket rate limiter with first-class
support for deployments behind reverse proxies (nginx, Caddy, AWS ALB, etc.).

---

## Quick start

The simplest way to add rate limiting is via `WithDefaultMiddleware`, which
wires a conservative limiter automatically (10 RPS, burst 20, no trusted
proxies):

```go
app := flow.New("my-app", flow.WithDefaultMiddleware())
```

To customise the limits use `WithRateLimit`:

```go
app := flow.New("my-app",
    flow.WithRateLimit(50, 100), // 50 RPS sustained, burst up to 100
)
```

---

## API reference

### `RateLimitOptions`

```go
type RateLimitOptions struct {
    // RPS is the sustained request rate per second per client IP.
    // Zero or negative disables limiting (no-op middleware returned).
    RPS int

    // Burst is the maximum burst size above RPS.
    Burst int

    // TrustedProxies is the list of CIDR ranges whose X-Forwarded-For
    // header is trusted. Empty (the default) means XFF is always ignored
    // and RemoteAddr is used as the client IP — the safe default.
    TrustedProxies []*net.IPNet
}
```

### `RateLimitMiddleware(rps, burst int) Middleware`

Returns a middleware with no trusted proxies. Safe for direct internet
exposure — `X-Forwarded-For` is completely ignored.

```go
app.Use(flow.RateLimitMiddleware(20, 40))
```

### `RateLimitMiddlewareWithOptions(opts RateLimitOptions) Middleware`

Full-featured variant. Use this when deploying behind a reverse proxy.

```go
proxies := flow.MustParseCIDRs([]string{"10.0.0.0/8"})

app.Use(flow.RateLimitMiddlewareWithOptions(flow.RateLimitOptions{
    RPS:            50,
    Burst:          100,
    TrustedProxies: proxies,
}))
```

### `WithRateLimit(rps, burst int) Option`

`App` option shorthand for `RateLimitMiddleware`.

```go
app := flow.New("my-app", flow.WithRateLimit(50, 100))
```

### `WithRateLimitOptions(opts RateLimitOptions) Option`

`App` option shorthand for `RateLimitMiddlewareWithOptions`.

```go
app := flow.New("my-app", flow.WithRateLimitOptions(flow.RateLimitOptions{
    RPS:            50,
    Burst:          100,
    TrustedProxies: flow.MustParseCIDRs([]string{"10.0.0.0/8"}),
}))
```

### `MustParseCIDRs(cidrs []string) []*net.IPNet`

Parses CIDR strings and **panics** on the first invalid entry. Intended for
package-level `var` blocks and test setup where a bad CIDR is a programming
error.

```go
var internalProxies = flow.MustParseCIDRs([]string{
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",
})
```

### `ParseCIDRs(cidrs []string) ([]*net.IPNet, error)`

Non-panicking variant. Use this in config-loading paths where CIDRs come from
user input or environment variables.

```go
proxies, err := flow.ParseCIDRs(cfg.TrustedProxyCIDRs)
if err != nil {
    return fmt.Errorf("invalid trusted proxy CIDR: %w", err)
}
```

---

## Trusted proxy model

### Why it matters

`X-Forwarded-For` (XFF) is a **client-controlled header**. Any HTTP client
can set it to an arbitrary value:

```
X-Forwarded-For: 1.2.3.4, 5.6.7.8
```

Trusting XFF unconditionally lets attackers trivially bypass per-IP rate
limiting by cycling the header value with each request.

### How Flow resolves the real client IP

Flow uses a **right-to-left walk** of the XFF chain, stopping at the first IP
that is **not** in the trusted proxy set:

```
Request path:    client → proxy-A (10.0.0.1) → proxy-B (10.0.0.2) → app
X-Forwarded-For: 203.0.113.5, 10.0.0.1
RemoteAddr:      10.0.0.2:54321

TrustedProxies:  10.0.0.0/8

Resolution:
  1. RemoteAddr peer = 10.0.0.2  → trusted ✓  (inspect XFF)
  2. XFF right-most = 10.0.0.1   → trusted ✓  (keep walking)
  3. XFF next       = 203.0.113.5 → NOT trusted ✗  → real client IP
```

If `TrustedProxies` is **empty** (the default), step 1 always fails and
`RemoteAddr` is used directly — XFF is never read.

### Default: no trusted proxies

```go
// Safe default — ignores X-Forwarded-For entirely.
flow.WithRateLimit(10, 20)
```

Use this when:
- The app is directly internet-facing (no load balancer).
- You are unsure of your proxy topology.

### Single reverse proxy

```go
// nginx, Caddy, or similar on the same host or private subnet.
flow.WithRateLimitOptions(flow.RateLimitOptions{
    RPS:   50,
    Burst: 100,
    TrustedProxies: flow.MustParseCIDRs([]string{
        "127.0.0.1/32",   // loopback (same host)
        "::1/128",        // IPv6 loopback
    }),
})
```

### Cloud load balancer (AWS ALB / GCP LB)

ALBs add the client IP as the left-most XFF entry and originate from a known
IP range. Prefer using the cloud provider's documented CIDR list:

```go
// Example: restrict to ALB IPs in a private VPC subnet.
flow.WithRateLimitOptions(flow.RateLimitOptions{
    RPS:   100,
    Burst: 200,
    TrustedProxies: flow.MustParseCIDRs([]string{
        "10.0.0.0/8",
        "172.16.0.0/12",
    }),
})
```

### Loading from environment / config

```go
func rateLimitFromConfig(cfg *AppConfig) flow.Option {
    proxies, err := flow.ParseCIDRs(cfg.TrustedProxyCIDRs)
    if err != nil {
        // fail fast at startup — bad config is a deployment error
        log.Fatalf("trusted proxy config invalid: %v", err)
    }
    return flow.WithRateLimitOptions(flow.RateLimitOptions{
        RPS:            cfg.RateLimitRPS,
        Burst:          cfg.RateLimitBurst,
        TrustedProxies: proxies,
    })
}
```

---

## Memory safety

The rate limiter maintains a per-IP token bucket in a closure-local map (not a
global). A background goroutine evicts entries that have been idle for longer
than **5 minutes**, so memory is bounded to `O(unique active clients)` rather
than growing unboundedly over the lifetime of the process.

The eviction goroutine is started lazily when the middleware handles its first
request and stops when the last request context is done or when the process
exits.

---

## Defaults used by `WithDefaultMiddleware`

| Constant | Value | Meaning |
|----------|-------|---------|
| `DefaultRateLimitRPS` | `10` | 10 requests per second per IP |
| `DefaultRateLimitBurst` | `20` | burst up to 20 requests |
| `TrustedProxies` | `nil` | XFF ignored; `RemoteAddr` always used |

These are intentionally conservative to protect new apps out of the box while
avoiding surprising throttling during development. Override them with
`WithRateLimitOptions` for production deployments.

---

## Security checklist

- [ ] Do **not** set `TrustedProxies` unless you are behind a reverse proxy
  with a known, fixed IP range.
- [ ] Prefer `ParseCIDRs` (returns error) over `MustParseCIDRs` (panics) when
  CIDRs come from user-supplied configuration.
- [ ] Review your cloud provider's documented IP ranges and restrict
  `TrustedProxies` to exactly those ranges — do not use `0.0.0.0/0`.
- [ ] Combine rate limiting with `BodyLimitMiddleware` for full DoS
  protection (see [`security.md`](security.md)).

---

## See also

- [`security.md`](security.md) — secure headers, session hardening, CSRF
- [`getting-started.md`](getting-started.md) — full App setup walkthrough
- [`architecture.md`](architecture.md) — middleware stack ordering
