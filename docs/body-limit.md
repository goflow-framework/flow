# Body Size Limiting

Flow ships `BodyLimitMiddleware` to cap incoming request body sizes and
protect against request-body denial-of-service (DoS) attacks. It is wired
automatically by `WithDefaultMiddleware` at a 4 MiB default.

---

## Quick start

`WithDefaultMiddleware` already includes a 4 MiB body limit. No extra
configuration is needed for typical apps:

```go
app := flow.New("my-app", flow.WithDefaultMiddleware())
```

To set a custom limit use `WithBodyLimit`:

```go
app := flow.New("my-app",
    flow.WithBodyLimit(1 << 20), // 1 MiB
)
```

---

## API reference

### `DefaultBodyLimitBytes`

```go
const DefaultBodyLimitBytes int64 = 4 << 20 // 4 MiB
```

The default limit used by `WithDefaultMiddleware`. Generous enough for typical
JSON and form payloads while blocking trivial DoS attempts.

### `BodyLimitMiddleware(maxBytes int64) Middleware`

Returns a middleware that caps incoming request bodies at `maxBytes`. The
limit is enforced lazily via `http.MaxBytesReader` — the error surfaces when
the body is first read (e.g. inside `BindJSON`, `BindForm`, or
`r.ParseMultipartForm`).

```go
// 512 KiB limit on a specific handler group
app.Use(flow.BodyLimitMiddleware(512 << 10))
```

A zero or negative value disables the limit and returns a no-op middleware:

```go
// Explicitly disable (not recommended in production)
app.Use(flow.BodyLimitMiddleware(0))
```

### `WithBodyLimit(maxBytes int64) Option`

`App` constructor option — registers `BodyLimitMiddleware` during app setup.

```go
app := flow.New("my-app", flow.WithBodyLimit(2<<20)) // 2 MiB
```

### `IsBodyTooLarge(err error) bool`

Reports whether `err` (or any error in its chain) was produced by
`http.MaxBytesReader` when the body exceeded the limit. Use this at
`BindJSON` / `BindForm` call-sites to return a precise 413 response instead
of a generic 400:

```go
func (c *UploadController) Create(ctx *flow.Context) {
    var payload UploadRequest
    if err := ctx.BindJSON(&payload); err != nil {
        if flow.IsBodyTooLarge(err) {
            ctx.Error(http.StatusRequestEntityTooLarge, "request body too large")
            return
        }
        ctx.Error(http.StatusBadRequest, "invalid request body")
        return
    }
    // ... handle payload
}
```

---

## How the limit is enforced

`BodyLimitMiddleware` wraps `r.Body` with `http.MaxBytesReader` before
calling the next handler. The reader enforces the cap **lazily** — bytes are
counted as the body is consumed, not upfront. This means:

1. The middleware itself does **not** buffer or read the body.
2. The limit fires the first time a handler (or binding helper) reads past
   `maxBytes`.
3. The read returns `*http.MaxBytesError` which `IsBodyTooLarge` detects.

```
Request arrives
      │
      ▼
BodyLimitMiddleware        ← wraps r.Body with MaxBytesReader(maxBytes)
      │
      ▼
  handler / BindJSON       ← reads body; error returned if > maxBytes
      │
      ▼
  IsBodyTooLarge(err)      ← true  →  respond 413
                           ← false →  respond 400 or handle normally
```

> **Important:** Register `BodyLimitMiddleware` **before** any middleware that
> reads the body (CSRF token parsing, JSON binding, multipart parsing). If the
> body is read before the limit middleware runs, the limit has no effect.
> `WithDefaultMiddleware` already registers it in the correct order.

---

## Choosing a limit

| Use case | Suggested limit |
|----------|----------------|
| Pure JSON APIs | `256 KiB` – `1 MiB` |
| Forms (no files) | `1 MiB` |
| Single file upload | `10 MiB` – `50 MiB` |
| Multi-file / large uploads | Disable global limit; set per-route limit |
| Default (mixed app) | `4 MiB` (`DefaultBodyLimitBytes`) |

For routes that legitimately need large bodies (file uploads), disable the
global limit and apply a higher per-route limit:

```go
// Global conservative limit
app := flow.New("my-app", flow.WithBodyLimit(256<<10))

// Per-route override for file uploads
r := flow.NewRouter(app)
r.Post("/files", func(c *flow.Context) {
    // Raise limit for this specific handler only
    http.MaxBytesReader(c.ResponseWriter(), c.Request().Body, 50<<20)
    // ... handle upload
})
```

---

## Recipes

### JSON API with strict body limit

```go
app := flow.New("api",
    flow.WithLogger(logger),
    flow.WithRequestID(""),
    flow.WithBodyLimit(256<<10), // 256 KiB — tight for pure JSON
    flow.WithMetrics(),
)
```

### File upload endpoint alongside a strict global limit

```go
app := flow.New("app", flow.WithBodyLimit(512<<10)) // 512 KiB default

r := flow.NewRouter(app)

// Regular JSON endpoint — inherits 512 KiB global limit
r.Post("/api/posts", postsController.Create)

// Upload endpoint — applies its own larger limit before reading
r.Post("/api/uploads", func(c *flow.Context) {
    r.Request().Body = http.MaxBytesReader(
        c.ResponseWriter(), c.Request().Body, 25<<20, // 25 MiB
    )
    if err := c.Request().ParseMultipartForm(25 << 20); err != nil {
        if flow.IsBodyTooLarge(err) {
            c.Error(http.StatusRequestEntityTooLarge, "file too large (max 25 MiB)")
            return
        }
        c.Error(http.StatusBadRequest, "invalid upload")
        return
    }
    // ... process file
})
```

### Handling the error in a custom error handler

```go
app.SetErrorHandler(func(ctx *flow.Context, err error) {
    if flow.IsBodyTooLarge(err) {
        ctx.JSON(http.StatusRequestEntityTooLarge, map[string]string{
            "error": "request body exceeds the maximum allowed size",
        })
        return
    }
    ctx.JSON(http.StatusInternalServerError, map[string]string{
        "error": "internal server error",
    })
})
```

---

## Default middleware stack order

`WithDefaultMiddleware` registers the body limit **third** in the stack,
after `Recovery` and `RequestID` but before `SecureHeaders`, `CSRF`,
`RateLimitMiddleware`, and `LoggingMiddleware`. This ensures:

- The limit is applied before any middleware reads the body (CSRF, JSON
  binding).
- Panics from oversized bodies are caught by the outer `Recovery` middleware.

```
Recovery
  └─ RequestIDMiddleware
       └─ BodyLimitMiddleware   ← here
            └─ SecureHeaders
                 └─ CSRFMiddleware
                      └─ RateLimitMiddleware
                           └─ LoggingMiddleware
                                └─ MetricsMiddleware
                                     └─ handler
```

---

## See also

- [`rate-limiting.md`](rate-limiting.md) — per-IP rate limiting and trusted
  proxy configuration
- [`security.md`](security.md) — secure headers, session hardening, CSRF
- [`getting-started.md`](getting-started.md) — full App setup walkthrough
