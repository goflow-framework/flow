# ci/Makefile - developer helpers for linting

.PHONY: lint lint-typecheck

# Fast PR-time lint: disables type-aware linters so CI/PR feedback is fast
# and avoids export-data mismatches. Mirrors the command used in CI.
lint:
	docker run --rm -v "$(PWD)":/src -w /src \
		ghcr.io/undiegomejia/golangci-analyzer:1.24.11-2.8.0 \
		/bin/bash -lc 'export PATH=/usr/local/go/bin:/go/bin; /go/bin/golangci-lint run --no-config --disable-all --enable=govet,errcheck,gofmt,goimports,gosimple,unused,ineffassign,bodyclose --timeout=10m --out-format=colored-line-number'

# Deterministic type-aware lint: runs staticcheck/typecheck inside the
# pinned analyzer container and writes diagnostics into ./ci-export-typecheck
lint-typecheck:
	docker run --rm -v "$(PWD)":/src -w /src \
		-e ENABLE_TYPECHECK=1 -e CI_EXPORT_DIR=./ci-export-typecheck \
		ghcr.io/undiegomejia/golangci-analyzer:1.24.11-2.8.0 \
		/bin/bash -lc 'chmod +x .github/scripts/run-golangci-in-container.sh && ./.github/scripts/run-golangci-in-container.sh _typecheck'
