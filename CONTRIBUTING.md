# Contributing to Flow

Welcome — thank you for considering contributing. This document captures the core design principles, how to run tests, and an actionable PR checklist aligned with the project's goals: explicit APIs, small interfaces, security-by-default, and low runtime overhead.

Core design principles

- Explicit over implicit: prefer small, explicit APIs and opt-in behavior. Avoid magic conventions unless well-documented and opt-in.
- Composition over inheritance: prefer small interfaces and composition for extensibility.
- Interface-driven, small interfaces: design small interfaces (Router, JobQueue, SessionStore) that are easy to mock and replace.
- Zero-cost abstractions: avoid runtime reflection or allocations in hot paths; add abstractions that compile away where possible.
- Context-first APIs: accept context.Context in long-running or cancellable operations.
- Fail-fast for developer errors, graceful in runtime: detect misconfiguration early; production behavior should be robust and documented.
- Secure-by-default: CSRF protection, secure session cookies, and safe template usage should be default behaviors.
- Avoid global state: prefer passing dependencies via App options and ServiceRegistry.

Developer checklist (PRs)

- Small focused PR: keep changes scoped and easy to review.
- Tests: add unit tests for behavior and integration tests for cross-package changes. New features must include at least one test exercising the public API.
- Documentation: update README or `docs/` when behavior or public APIs change. Add examples under `examples/` where helpful.
- Linting & formatting: run `gofmt -w .` and ensure `staticcheck` is clean for new code.
- Backwards compatibility: document any API breaks and follow semver rules for public API changes.

How to run tests locally

Recommended quick commands (run in WSL or Linux):

```bash
# run all tests (may take longer)
go test ./... -v

# run tests for core packages only
go test ./pkg/... -v

# run without cache (useful when editing test helpers)
go test ./... -count=1
```

Generator tests

Generator integration tests compile generated code in a temp module. These tests are fragile across Go versions. The tests use absolute `replace` directives and pin the Go toolchain in the temporary module. If generator compile tests fail in CI, re-run locally with a matching Go version.

CI

- The repo runs `go vet`, `staticcheck`, `gofmt` checks and a matrix of Go versions in CI. Heavy static analysis runs in a dedicated job so it doesn't slow fast test feedback.

Plugin policy

- Plugins should be explicit: runtime registration via `plugin.Register(app, opts...)` is preferred over side-effectful `init()` registration, except for well-known standard plugins.
- Plugins must declare a semantic version and implement lifecycle hooks when background tasks are required. The framework validates compatible major versions where feasible.

Reporting issues & PR etiquette

- Use clear issue titles and steps to reproduce. Include `go version` and OS when relevant.
- For PRs, include a short summary of design choices and a link to any relevant design docs.

Thanks for contributing — we aim to keep Flow small, explicit, and pleasant to use. If you need help choosing an issue, look for labels `good-first-issue` or `help-wanted`.
