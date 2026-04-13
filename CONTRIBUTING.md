# Contributing to Flow

Thanks for your interest in contributing to Flow. This document summarizes the core design principles that guide development and the practical steps to make a contribution.

## Core design principles

- Explicitness: APIs should be clear and explicit. Prefer small, focused interfaces over large, implicit behavior.
- Small interfaces: Keep interfaces minimal and easy to implement. This reduces cognitive overhead for third-party extensions and tests.
- App-scoped state: Avoid global mutable state. Registries (services, plugins) are attached to a specific `*flow.App` instance so multiple apps can coexist in the same process.
- Plugin lifecycle: Plugins expose a canonical lifecycle (Init -> Mount -> Start -> Stop). Init and Mount run synchronously during registration; Start is run by the host during `App.Start()` and should return when the provided context is canceled; Stop is called during `App.Shutdown()`.
- Conservative versioning: The framework enforces major-version compatibility for plugins. Plugins must expose a semantic Version() and the major version must match the framework API major (see docs/architecture.md).
- Testability: Favor deterministic, small, and isolated tests. Avoid relying on global state or external services in unit tests; use integration tests sparingly and keep them stable with clear golden data.

## Developer Certificate of Origin (DCO)

By making a contribution to this project you certify that you agree to the
[Developer Certificate of Origin v1.1](https://developercertificate.org/):

> By making a contribution to this project, I certify that:
>
> (a) The contribution was created in whole or in part by me and I have the right to submit it under the
> open source license indicated in the file; or
>
> (b) The contribution is based upon previous work that, to the best of my knowledge, is covered under
> an appropriate open source license and I have the right under that license to submit that work with
> modifications, whether created in whole or in part by me, under the same open source license (unless
> I am permitted to submit under a different license), as indicated in the file; or
>
> (c) The contribution was provided directly to me by some other person who certified (a), (b) or (c)
> and I have not modified it.
>
> (d) I understand and agree that this project and the contribution are public and that a record of the
> contribution (including all personal information I submit with it, including my sign-off) is maintained
> indefinitely and may be redistributed consistent with this project or the open source license(s)
> involved.

### How to sign off

Add a `Signed-off-by` trailer to every commit using the `-s` / `--signoff` flag:

```sh
git commit -s -m "feat: add awesome feature"
```

This produces a commit message like:

```
feat: add awesome feature

Signed-off-by: Your Name <you@example.com>
```

The name and e-mail must match your Git identity (`git config user.name` /
`git config user.email`). Pull requests without a `Signed-off-by` line on
every commit will not be merged.

**Tip — sign off retroactively** (before pushing a new branch):

```sh
git rebase HEAD~<number-of-commits> --signoff
```

## How to contribute

1. Fork the repository and create a branch with a clear name, e.g. `feat/your-feature` or `fix/some-bug`.
2. Run tests locally: `go test ./...` and make sure they pass.
3. Follow the existing formatting and linting rules. Run `gofmt` and any project linters.
4. Add or update tests for any behavior you change. For public API changes, include examples and docs updates.
5. Open a pull request describing the change, why it is needed, and any migration steps.

## Review checklist (PR authors)

- Does the change preserve the core design principles listed above?
- Are public APIs documented in `docs/`?
- Are tests present for new/changed behavior?
- Does the code compile and do the tests pass on `go test ./...`?
- Does every commit carry a `Signed-off-by` line (`git commit -s`)?

Maintainers will review and ask for changes if necessary. For larger design changes, open an issue first to discuss the approach.

## Contact and help

If you need help contributing, open an issue describing what you want to do and tagging it with `help wanted`. For sensitive matters (security, legal), contact the project owners via the repository settings.

---

## Local tooling and linters (recommended)

To make contributions easier and consistent across the project, please install the recommended tooling locally. These commands are quick helpers — CI will also run the same checks.

- Install golangci-lint (used by CI):

```sh
# linux / macOS
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0
```

- Install a hot-reload tool for local development (optional). `air` is a popular choice:

```sh
go install github.com/cosmtrek/air@latest
```

### Running checks locally

- Run gofmt check:

```sh
gofmt -l .
```

- Lint and vet locally (fast):

```sh
go vet ./...
golangci-lint run --timeout=5m
```

- Full check (CI parity):

```sh
go mod download
go vet ./...
golangci-lint run --timeout=10m
go test ./... -v
```

### Makefile targets

We provide convenient Makefile targets (see `Makefile` in the repo root):

- `make dev` — run the app locally with hot reload (requires `air` or similar).
- `make test` — run `go vet`, `golangci-lint`, and `go test ./...`.
- `make ci` — run the same steps CI runs (downloads modules, lint, vet, tests).

If you prefer other tools, the important thing is to keep the output consistent with CI so your PRs get the same feedback developers see in the pipeline.

---

## Further notes

- For changes that affect the public plugin API, please document the compatibility constraints and add a migration note in `CHANGELOG.md`.
- If your change depends on external services (databases, SMTP, etc.), prefer to keep the unit tests isolated and provide an integration test guarded by a build tag or documented environment variables.

Thank you for contributing!
