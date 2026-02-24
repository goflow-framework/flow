SHELL := /bin/bash
.PHONY: dev test ci

dev:
	@echo "Starting dev server (hot-reload if 'air' is installed)..."
	@command -v air >/dev/null 2>&1 && air || go run ./cmd/flow

test:
	@echo "Running vet, linters and tests..."
	go vet ./...
	-golangci-lint run --timeout=5m
	go test ./... -v

ci:
	@echo "CI parity: download modules, vet, lint and test"
	go mod download
	go vet ./...
	-golangci-lint run --timeout=10m
	go test ./... -v
