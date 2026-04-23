# Security middleware and hardening

Flow provides a small set of conservative, opt-in helpers to harden HTTP
applications without surprising defaults. These are intended to be easy to
enable during app bootstrap and to avoid breaking existing applications.

SecureHeaders
--------------

Use `flow.SecureHeaders()` or the convenience `flow.WithSecureDefaults(app)` to
enable a set of headers that improve security posture:

- Strict-Transport-Security (HSTS) when requests are observed over TLS
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin (default)

Content-Security-Policy is intentionally not set by default to avoid breaking
existing inline scripts/styles. Provide a CSP via options when you require it.

CSRF
----

Flow includes a small, session-backed CSRF middleware `CSRFMiddleware()` which
ensures a per-session token exists and validates it for unsafe HTTP methods.
It supports the `X-CSRF-Token` header and the `_csrf` form field. The middleware
requires the `SessionManager` to be present in the middleware chain.

Session cookie hardening
------------------------

Some session libraries do not mark cookies as Secure or set a SameSite policy.
Use the `SessionCookieHardening` middleware to append conservative attributes
(`Secure; SameSite=Lax`) to outgoing `Set-Cookie` headers when they are
missing. This is useful when enabling HTTPS for existing applications without
changing session manager implementations.

## Migration: hardening cookies for existing apps

If you have an existing application or a third-party session manager that does
not set `Secure` or `SameSite` attributes on cookies, you can harden cookies
without changing the session implementation. Simply register the session
middleware first, then add `SessionCookieHardening()` after it. The middleware
will append conservative attributes to outgoing `Set-Cookie` headers.

```go
// existing session middleware (third-party or custom)
app.Use(existingSessionMiddleware)
// append conservative attributes when missing
app.Use(flow.SessionCookieHardening())
```

If you control the `SessionManager` used by your app (the Flow-provided
`SessionManager`), you can enable default secure cookie flags during
initialization:

```go
sm := flow.DefaultSessionManager()
sm.ApplySecureCookieDefaults() // marks cookies Secure and SameSite=Lax
app.Use(sm.Middleware())
```

This allows a gradual migration: enable `SessionCookieHardening` immediately to
protect users, and later switch session manager configuration to emit secure
cookies directly when ready.

Examples
--------

Enable secure defaults during bootstrap:

```go
app := flow.New("myapp")
flow.WithSecureDefaults(app)
// Ensure session middleware runs before CSRF and hardening
sm := flow.DefaultSessionManager()
app.Use(sm.Middleware())
app.Use(flow.CSRFMiddleware())
app.Use(flow.SessionCookieHardening())
```

JSON API CSRF
------------

For JSON APIs or single-page apps that transmit the CSRF token via an HTTP
header, Flow includes a small helper `CSRFMiddlewareJSON()` which validates
the `X-CSRF-Token` header for unsafe methods on requests with a JSON
content-type. Use this alongside the session middleware to protect JSON
endpoints:

```go
sm := flow.DefaultSessionManager()
app.Use(sm.Middleware())
// register JSON CSRF protection for API routes
app.Use(flow.CSRFMiddlewareJSON())
```

Notes
-----

- All middleware here is opt-in. They are intentionally conservative to avoid
  breaking applications when enabled.
- Review Content-Security-Policy choices carefully; restrictive CSPs may
  require application changes.

---

## See also

- [`rate-limiting.md`](rate-limiting.md) — per-IP rate limiting, trusted proxy
  configuration, and `X-Forwarded-For` security model
- [`body-limit.md`](body-limit.md) — `BodyLimitMiddleware`, `IsBodyTooLarge`,
  per-route overrides, and choosing a limit for your use case
