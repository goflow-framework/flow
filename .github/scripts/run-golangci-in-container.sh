#!/bin/sh
set -euo pipefail
# Usage: run-golangci-in-container.sh [suffix]
# Writes diagnostics into ./ci-export-typecheck and records the golangci exit code.

SUFFIX="${1:-}"
OUTDIR="./ci-export-typecheck"
GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v1.59.0/golangci-lint-1.59.0-linux-amd64.tar.gz"

mkdir -p "$OUTDIR" /tmp/gomodcache /tmp/gocache
export PATH=/usr/local/go/bin:/go/bin:$PATH
export GOFLAGS='-mod=mod -buildvcs=false'

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
