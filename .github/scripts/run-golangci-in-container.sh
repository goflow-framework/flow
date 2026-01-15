#!/bin/sh
set -euo pipefail
# run-golangci-in-container.sh — tiny README
#
# Purpose:
#   Run diagnostics and execute golangci-lint inside the job-level Go container
#   in a repeatable way. The script writes useful debug artifacts into
#   ./ci-export-typecheck and exits with the golangci-lint exit code so the
#   calling workflow can gate behavior (non-blocking vs blocking runs).
#
# Usage:
#   ./github/scripts/run-golangci-in-container.sh [suffix]
#
#   - Optional `suffix` (for example `_blocking`) is appended to output file
#     names so parallel runs (non-blocking and blocking) don't overwrite each
#     other's diagnostics.
#
# Why we clear /usr/local/go/pkg/*:
#   The Go toolchain writes compiled package export data under GOROOT/pkg
#   (the standard library) and in module caches. If those compiled artifacts
#   were produced by a different Go toolchain or in a different environment
#   (for example, the runner vs the container), golangci-lint's typecheck
#   (and other analyzers) can fail with "unsupported version" or "stale"
#   import/export-data errors. Removing `/usr/local/go/pkg/*` inside the
#   ephemeral container forces the Go toolchain to rebuild any necessary
#   stdlib artifacts with the container's Go version, avoiding mismatches.
#
#   This is safe in an ephemeral container (we only remove files inside the
#   container's GOROOT) and prevents hard-to-debug export-data issues.
#
# Writes diagnostics into ./ci-export-typecheck and records the golangci exit code.

SUFFIX="${1:-}"
OUTDIR="./ci-export-typecheck"
GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v1.59.0/golangci-lint-1.59.0-linux-amd64.tar.gz"

mkdir -p "$OUTDIR" /tmp/gomodcache /tmp/gocache
export PATH=/usr/local/go/bin:/go/bin:$PATH
export GOFLAGS='-mod=mod -buildvcs=false'
# Ensure GOROOT is set when the container's Go is available. Some images
# ship a trimmed 'go' binary or have GOROOT cleared; setting GOROOT to the
# container's installation path (/usr/local/go) prevents "go: cannot find
# GOROOT directory: 'go' binary is trimmed and GOROOT is not set" errors.
if [ -x /usr/local/go/bin/go ]; then
  export GOROOT=/usr/local/go
fi

# Clear caches and compiled stdlib packages that may cause export-data mismatches
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go clean -cache -modcache -testcache -i || true
rm -rf /usr/local/go/pkg/* || true
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download || true

# Collect diagnostics
echo "$PATH" > "$OUTDIR/path${SUFFIX}.txt" 2>/dev/null || true
which go > "$OUTDIR/which_go${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go version > "$OUTDIR/go_version${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go env -json > "$OUTDIR/go_env${SUFFIX}.json" 2>&1 || true
/usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps${SUFFIX}.json" 2>&1 || true
/usr/local/go/bin/go list -json sync/atomic > "$OUTDIR/sync_atomic${SUFFIX}.json" 2>&1 || true
ls -la /usr/local/go/bin > "$OUTDIR/goroot_bin_ls${SUFFIX}.txt" 2>&1 || true
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_ls${SUFFIX}.txt" 2>&1 || true

# Download and run golangci-lint from /tmp to avoid mv/timing/permission issues
rc=0
curl -sSfL "$GOLANGCI_URL" | tar -xz -C /tmp || rc=2
if [ "$rc" -eq 0 ]; then
  /tmp/golangci-lint-1.59.0-linux-amd64/golangci-lint run --config .golangci.yml --enable typecheck ./... > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || rc=$?
fi
echo "$rc" > "$OUTDIR/golangci_exit_code${SUFFIX}" || true
exit "$rc"
