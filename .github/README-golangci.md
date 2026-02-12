Reproducing golangci-lint runs locally

This repository uses a two-stage approach for linting:

- Fast PR-time linting: runs quick, non-type-aware linters to provide fast feedback
  and avoid flaky export-data/typecheck failures on GitHub runners.

- Deterministic type-aware checks: `staticcheck` and `typecheck` run in a
  pinned analyzer container (or on main via workflow dispatch) where the Go
  toolchain and compiled stdlib are controlled.

Run quick, safe linters locally (matches PR behavior):

```bash
# Use the analyzer container but disable type-aware linters to match PR gate
docker run --rm -v "$(pwd)":/src -w /src \
  ghcr.io/undiegomejia/golangci-analyzer:1.24.11-2.8.0 \
  golangci-lint run --timeout=10m --disable=typecheck,staticcheck --out-format=colored-line-number
```

Run the deterministic type-aware checks locally (reproduces the CI typecheck job):

```bash
# Run inside the pinned analyzer container and enable type-aware linters.
# The helper script will prepare caches and rebuild stdlib where required.
docker run --rm -v "$(pwd)":/src -w /src ghcr.io/undiegomejia/golangci-analyzer:1.24.11-2.8.0 \
  /bin/bash -c "ENABLE_TYPECHECK=1 CI_EXPORT_DIR=./ci-export-typecheck ./ .github/scripts/run-golangci-in-container.sh _typecheck"
```

Notes
- If you see `could not import sync/atomic (unsupported version: ...)` it means
  the analyzer binary and the Go stdlib/compiled packages were built with
  incompatible toolchain versions. Use the containerized typecheck path for a
  deterministic run.
- For CI we run the deterministic job only on `push` to `main` and via
  `workflow_dispatch` to avoid blocking PR feedback on flaky host caches.
