# Contributing to Flow

Thanks for your interest in contributing to Flow. This document summarizes the core design principles that guide development and the practical steps to make a contribution.

Core design principles

- Explicitness: APIs should be clear and explicit. Prefer small, focused interfaces over large, implicit behavior.
- Small interfaces: Keep interfaces minimal and easy to implement. This reduces cognitive overhead for third-party extensions and tests.
- App-scoped state: Avoid global mutable state. Registries (services, plugins) are attached to a specific `*flow.App` instance so multiple apps can coexist in the same process.
- Plugin lifecycle: Plugins expose a canonical lifecycle (Init -> Mount -> Start -> Stop). Init and Mount run synchronously during registration; Start is run by the host during `App.Start()` and should return when the provided context is canceled; Stop is called during `App.Shutdown()`.
- Conservative versioning: The framework enforces major-version compatibility for plugins. Plugins must expose a semantic Version() and the major version must match the framework API major (see docs/architecture.md).
- Testability: Favor deterministic, small, and isolated tests. Avoid relying on global state or external services in unit tests; use integration tests sparingly and keep them stable with clear golden data.

How to contribute

1. Fork the repository and create a branch with a clear name, e.g. `feat/your-feature` or `fix/some-bug`.
2. Run tests locally: `go test ./...` and make sure they pass.
3. Follow the existing formatting and linting rules. Run `gofmt` and any project linters.
4. Add or update tests for any behavior you change. For public API changes, include examples and docs updates.
5. Open a pull request describing the change, why it is needed, and any migration steps.

Review checklist (PR authors)

- Does the change preserve the core design principles listed above?
- Are public APIs documented in `docs/`?
- Are tests present for new/changed behavior?
- Does the code compile and do the tests pass on `go test ./...`?

Maintainers will review and ask for changes if necessary. For larger design changes, open an issue first to discuss the approach.

Contact and help

If you need help contributing, open an issue describing what you want to do and tagging it with `help wanted`. For sensitive matters (security, legal), contact the project owners via the repository settings.
