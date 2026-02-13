Title: ci: pin Go 1.24.11 for generator tests

Why
---
Generator integration tests build and execute generated projects. They require a stable Go toolchain that matches the repo `go` directive and the toolchain used locally during development. The repo `go.mod` contains `go 1.24.0` and `toolchain go1.24.11`. Pinning the generator-tests job to `go 1.24.11` avoids failures caused by runner defaults or tooling changes.

What
----
This PR pins the `generator-compile` GitHub Actions job to use `actions/setup-go@v4` with `go-version: '1.24.11'` and updates the cache key used in that job to include the pinned version.

Verification
------------
- CI: the `generator-compile` job should pass on this branch.
- Local: `go test ./internal/generator -v` should pass when running with Go 1.24.11.

Notes
-----
If you prefer a different patch (e.g., pinning all jobs to a specific patch version), I included a small patch that changes only the generator job. We can extend the pin across other jobs if desired.
