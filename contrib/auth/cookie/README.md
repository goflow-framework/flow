Contrib: Cookie-based auth middleware
===================================

This small contrib package provides a convenience middleware that resolves a
user identity from the framework `SessionManager` and stores a resolved user
value into the request context for downstream handlers.

Usage
-----

1. Register the session middleware on the `App`.
2. Register the cookie auth middleware after the session middleware.
3. Provide a lookup function that maps the session identifier to your domain user.

Example:

```go
sm := flow.DefaultSessionManager()
app := flow.New("auth-demo")
app.Use(sm.Middleware())
// lookup resolves a session uid to a user value (could query DB)
lookup := func(id interface{}) (interface{}, error) { return findUser(id) }
app.Use(cookie.Middleware(sm, "uid", lookup))

// In handlers retrieve the resolved user:
u, ok := cookie.FromContext(r.Context())
if ok { // use u }
```

Notes
-----
- The middleware is intentionally small and application-agnostic: you decide
  how to resolve session identifiers into domain users (DB call, cache, etc.).
- The session middleware must run before the cookie auth middleware so the
  session value is available on the request context.
