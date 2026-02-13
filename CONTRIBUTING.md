# Contributing to Flow

Thanks for your interest in contributing to Flow. This document explains the
minimal workflow and quality expectations for contributions. It is intentionally
short — follow the code and tests as the ground truth.

## Getting started

- Fork the repository and create a feature branch from `main`.
- Run tests locally: `go test ./...` and format: `gofmt -w .`.
- Use `go test ./pkg/flow -race` for concurrency-sensitive changes.

## Code style

- Keep public APIs small and explicit.
- Prefer composition and small interfaces over large structs.
- Avoid global mutable state.

## Tests

- Add unit tests for new behavior and integration tests when behaviour spans
  multiple packages.
- CI requires `gofmt`, `go vet`, and `staticcheck` to pass.

## Pull Requests

- Open PRs against the `main` branch.
- Provide a clear description with motivation and a short summary of changes.
- Add `Fixes #<issue>` if the PR closes an issue.

## Security

- Do not commit secrets; use environment variables or a local secrets file.

Thank you for contributing! Please be patient during review — we prefer
quality over speed.
