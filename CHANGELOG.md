# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Secure default middleware (`SecureHeaders`, `SessionCookieHardening`) and helper `WithSecureDefaults(*App)` to register conservative security headers and session cookie defaults. Added unit tests, an example (`examples/security_demo`) and docs (`docs/security.md`).
- Middleware integration assertion tests covering Recovery, RequestID, Metrics, Timeout, stack ordering, and concurrency safety (`pkg/flow/middleware_integration_test.go`).
- `docs/body-limit.md` — full API reference for `BodyLimitMiddleware`, `WithBodyLimit`, `IsBodyTooLarge`, `DefaultBodyLimitBytes`; includes lazy-enforcement explanation, limit sizing guide, per-route override recipes, and middleware stack order diagram.
- `docs/rate-limiting.md` — full API reference and security guide for `RateLimitMiddleware`, `RateLimitMiddlewareWithOptions`, `WithRateLimit`, `WithRateLimitOptions`, `MustParseCIDRs`, `ParseCIDRs`, and the trusted-proxy `X-Forwarded-For` security model.
- Cross-links between `docs/rate-limiting.md` ↔ `docs/body-limit.md` in their respective See Also sections.
- `README.md` install section: added `go get github.com/goflow-framework/flow@v0.10.0` snippet and `pkg.go.dev` link for the new module path.

### Fixed
- **`MigrationRunner` PostgreSQL placeholder bug**: the internal `flow_migrations` tracking queries used SQLite/MySQL `?` placeholders, causing silent failures against PostgreSQL. `MigrationRunner` now has a `Dialect` field (`DialectSQLite` / `DialectPostgres`); the `placeholder()` method returns `?` or `$1` accordingly. Zero-value `Dialect` preserves existing SQLite behaviour, so no existing code is broken. The CLI (`db migrate`, `db rollback`, `db status`) auto-detects the dialect from the DSN prefix. Three new unit tests cover the `placeholder()` logic and explicit-dialect apply/rollback round-trip.

### Notes
- Migration: enabling `WithSecureDefaults(app)` is opt-in. To avoid breaking existing setups, `SessionCookieHardening` can be enabled first to append conservative attributes on outgoing `Set-Cookie` headers; migrate session manager settings (call `ApplySecureCookieDefaults()`) once you confirm traffic and clients are compatible.

---

## [0.10.0] — 2026-04-23

### Security
- **AES-256-GCM session cookie encryption** — session values are now authenticated and encrypted at rest (#164).
- **Body size limit middleware** (`BodyLimitMiddleware`) — prevents request-body DoS attacks (#136).
- **Trusted-proxy validation for rate limiter** — `X-Forwarded-For` is no longer blindly trusted; per-IP maps use TTL eviction to prevent unbounded memory growth (#138).
- **Session cookies default to `Secure=true, SameSite=Lax`** (#142).
- **CSRF middleware** now returns HTTP 500 (not a silent empty token) on `crypto/rand` failure (#143).
- **Go toolchain bumped to 1.25.9** to resolve stdlib CVEs (#137).

### Added
- `/healthz` and `/livez` health-check endpoints registered automatically on `App` (#152).
- `SlogLogger` adapter bridging `log/slog` (Go 1.21+) to the `Logger` interface (#153).
- `GetServiceTyped[T]` / `RegisterServiceTyped[T]` generic helpers on `App` (#157).
- `BindForm`, `BindQuery`, `Validate` methods on `Context` (#133).
- Typed `Config`, `FLOW_ENV` environment variable, and `WithConfig` option (#126).
- `flow new` scaffold now generates test stubs alongside application code (#154).
- Per-plugin configurable deadline for `Start` goroutines during shutdown (#151).
- Getting-started walkthrough at `docs/getting-started.md` (#135).
- DCO sign-off requirement and instructions in `CONTRIBUTING.md` (#160).
- `SECURITY.md` with vulnerability disclosure policy and response timeline (#159).

### Fixed
- `errors.Is` used consistently instead of `==` for `sql.ErrNoRows` detection (#158).
- `hasJSONContentType` uses `mime.ParseMediaType` for correct MIME handling (#155).
- `Context.W` and `Context.R` are now unexported to prevent external mutation (#141).
- Package-level `validate` variable eliminated; no more data race under parallel requests (#140).
- DB connection pool defaults configured to prevent connection storms and leaks (#139).
- Plugin shutdown ordering corrected — plugins stop in reverse registration order (#163).
- `BoundedExecutor` replaced ticker-poll with `sync.WaitGroup`; fixes channel-close race (#162).
- `ViewManager.Render` uses `sync.Pool` to eliminate per-request `template.Clone()` (#156).
- `pkg/flow/api` → `pkg/flow` import cycle broken (#132).

### Refactored
- `App` struct fields grouped into labeled sections for readability (#161).
- `App` lifecycle extracted into `internal/app.Lifecycle` (#127).
- `paramsPool` moved from package-level global to per-`Router` field, eliminating cross-App pool sharing (#165).

---

## [0.9.0] — 2026-02-19

### Added
- Router DSL with named routes and per-route middleware (#100).
- Plugin governance and lifecycle (`Init`, `Start`, `Stop`) with long-term plugin guide (#95).
- Benchmarks for router and middleware with `pprof` integration (#86).
- `WithRedaction` for structured log field redaction (#99).
- `flow new`, `flow db migrate` and full CLI rename to `flow` (#130).
- ORM PostgreSQL factory and Bun adapter (#131).
- Phase-1 core runtime: `RequestGroup`, `Executor`, `ContextPool` (#104).

### Fixed
- CI hardening: `go vet`, `staticcheck`, `golangci-lint`, coverage gate, and artifact upload (#84, #134).
- Deprecated `strings.Title` replaced throughout (#73feb87).

---

## [0.1.0] — 2025 (initial public release)

- Initial MVC framework: router, middleware stack, view manager, migrations, CLI generator, ORM helpers.

