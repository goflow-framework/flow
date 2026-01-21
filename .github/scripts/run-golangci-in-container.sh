#!/bin/sh
set -euo pipefail
set -x
#!/usr/bin/env bash
# Use bash: this script relies on bash features (pipefail, better set handling)
# and is intended to run inside the pinned analyzer/container which provides
# a full shell. Using bash makes the script simpler and more robust.
set -euo pipefail
set -x
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
# Allow caller to override where diagnostics should be written (useful when
# the host binds a directory into the container at /ci-export-typecheck).
OUTDIR="${CI_EXPORT_DIR:-./ci-export-typecheck}"
GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v1.59.0/golangci-lint-1.59.0-linux-amd64.tar.gz"

mkdir -p "$OUTDIR" /tmp/gomodcache /tmp/gocache
# quick checkpoint so we know the container executed this script
# Guaranteed helper marker written immediately from inside the helper so
# CI artifacts include at least one in-container file even if the helper
# later fails. Also echo a short line to stdout so the host-captured
# docker_run.stdout shows activity from inside the container.
printf "%s\n" "container_started_from_helper: $(date --utc) uid=$(id -u 2>/dev/null || echo n/a) pid=$$" > "${OUTDIR}/container_started_from_helper${SUFFIX}.txt" 2>/dev/null || true
printf "%s\n" "helper-stdout-announce: helper started at $(date --utc) pid=$$" || true
echo "container_started: $(date) uid=$(id -u 2>/dev/null || echo n/a)" > "$OUTDIR/container_started${SUFFIX}.txt" 2>/dev/null || true

# Start a background heartbeat that writes to stderr (so the runner logs show
# activity even if tar streaming out later gets interrupted). We also keep a
# lightweight heartbeat file inside the output dir for post-mortem inspection.
HEARTBEAT_FILE="$OUTDIR/heartbeat${SUFFIX}.log"
(
  while :; do
    echo "HEARTBEAT: $(date) pid=$$ suffix=${SUFFIX}" >&2 || true
    echo "$(date)" >> "$HEARTBEAT_FILE" 2>/dev/null || true
    sleep 2
  done
) &
HEART_PID=$!
trap 'kill ${HEART_PID} 2>/dev/null || true' EXIT

export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export PATH=/usr/local/go/bin:/go/bin:$PATH
export GOFLAGS='-mod=mod -buildvcs=false'
# Ensure GOROOT is set when the container's Go is available. Some images
# ship a trimmed 'go' binary or have GOROOT cleared; setting GOROOT to the
# container's installation path (/usr/local/go) prevents "go: cannot find
# GOROOT directory: 'go' binary is trimmed and GOROOT is not set" errors.
if [ -x /usr/local/go/bin/go ]; then
  export GOROOT=/usr/local/go
fi

# Make sure CI_EXPORT_DIR is set and writable (fall back to the default mount)
: "${CI_EXPORT_DIR:=/ci-export-typecheck}"
mkdir -p "$CI_EXPORT_DIR" 2>/dev/null || true

# Early marker so the runner artifact tells us the helper started and could write
echo "helper_started: $(date --utc) $$" > "$CI_EXPORT_DIR/helper_started.txt" 2>/dev/null || true

# Always write a final exit code on exit so artifacts include it
_cleanup() {
  rc=$?
  echo "$rc" > "$CI_EXPORT_DIR/golangci_exit_code" 2>/dev/null || true
}
trap _cleanup EXIT

# ensure there's always at least a placeholder output file
: "${CI_EXPORT_DIR:=/ci-export-typecheck}"
if [ ! -s "$CI_EXPORT_DIR/golangci_typecheck.out" ]; then
  echo "no golangci output produced" > "$CI_EXPORT_DIR/golangci_typecheck.out" 2>/dev/null || true
fi

# Try to write an early marker so we can see the helper started and could write into CI_EXPORT_DIR
if [ -n "${CI_EXPORT_DIR:-}" ]; then
  mkdir -p "${CI_EXPORT_DIR}" 2>/dev/null || true
  echo "helper_started: $(date --utc) $$" > "${CI_EXPORT_DIR}/helper_started.txt" 2>/dev/null || true
fi

# Ensure we're running from the repository root inside the container. When the
# workspace is mounted with different ownership, git may complain about dubious
# ownership and commands like `go list` (which use the vcs) can fail. Force the
# current directory to the workspace and mark it as a safe directory for git.
if [ -n "${GITHUB_WORKSPACE:-}" ]; then
  cd "$GITHUB_WORKSPACE" || true
fi
if command -v git >/dev/null 2>&1; then
  # record the git user and config before we set safe.directory
  id > "$OUTDIR/id${SUFFIX}.txt" 2>/dev/null || true
  stat -c '%U %G %a %n' . > "$OUTDIR/pwd_stat${SUFFIX}.txt" 2>/dev/null || true
  git config --global --list > "$OUTDIR/git_global_config_before${SUFFIX}.txt" 2>/dev/null || true
  git config --global --add safe.directory "$(pwd)" || true
  git config --global --list > "$OUTDIR/git_global_config_after${SUFFIX}.txt" 2>/dev/null || true
fi

# Capture GOROOT pkg layout before we modify it
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_before${SUFFIX}.txt" 2>/dev/null || true

# Clear caches and compiled stdlib packages that may cause export-data mismatches
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go clean -cache -modcache -testcache -i || true
if [ -d /usr/local/go/pkg ]; then
  for entry in /usr/local/go/pkg/*; do
    base="$(basename "$entry")"
    # preserve 'tool' and 'include' directories which contain toolchain
    # binaries and header files required by the container's go runtime.
    if [ "$base" = "tool" ] || [ "$base" = "include" ]; then
      # preserve tool binaries
      continue
    fi
    rm -rf "$entry" || true
  done
fi
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download || true
  # Preserve the toolchain binaries and headers so the 'go' command
  # remains functional after the ultra-clean. Copy back 'tool' and
  # 'include' from the backup into the fresh pkg tree. This keeps the
  # ultra intent of removing compiled package export-data while not
  # removing required go toolchain binaries (avoids "no such tool
  # 'compile'" errors).
  if [ -d /tmp/goroot_pkg_backup/tool ]; then
    cp -a /tmp/goroot_pkg_backup/tool /usr/local/go/pkg/ || true
  fi
  if [ -d /tmp/goroot_pkg_backup/include ]; then
    cp -a /tmp/goroot_pkg_backup/include /usr/local/go/pkg/ || true
  fi
  

# Capture GOROOT pkg layout after cleanup
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_after${SUFFIX}.txt" 2>/dev/null || true

# Collect diagnostics
echo "$PATH" > "$OUTDIR/path${SUFFIX}.txt" 2>/dev/null || true
which go > "$OUTDIR/which_go${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go version > "$OUTDIR/go_version${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go env -json > "$OUTDIR/go_env${SUFFIX}.json" 2>&1 || true
/usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps${SUFFIX}.json" 2>&1 || true
ls -la /usr/local/go/bin > "$OUTDIR/goroot_bin_ls${SUFFIX}.txt" 2>&1 || true
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_ls${SUFFIX}.txt" 2>&1 || true

# Preflight: ensure we can import sync/atomic with the container's go toolchain.
# If this fails, write diagnostics and abort early so CI artifacts show the
# underlying `go list` failure instead of only golangci knock-on errors.
if /usr/local/go/bin/go list -json sync/atomic > "$OUTDIR/sync_atomic_preflight${SUFFIX}.json" 2>&1; then
  echo "sync_atomic_preflight: ok" > "$OUTDIR/sync_atomic_preflight_status${SUFFIX}.txt" 2>/dev/null || true
else
  echo "sync_atomic_preflight: failed" > "$OUTDIR/sync_atomic_preflight_status${SUFFIX}.txt" 2>/dev/null || true
  # Also keep the original failing output in the legacy filename for tools
  # that read sync_atomic*.json
  /usr/local/go/bin/go list -json sync/atomic > "$OUTDIR/sync_atomic${SUFFIX}.json" 2>&1 || true
  echo "preflight_failed: go list sync/atomic failed; aborting before golangci-lint" > "$OUTDIR/preflight_failed${SUFFIX}.txt" 2>/dev/null || true
  exit 2
fi

ls -la /usr/local/go/bin > "$OUTDIR/goroot_bin_ls${SUFFIX}.txt" 2>&1 || true
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_ls${SUFFIX}.txt" 2>&1 || true

# Marker to show the script reached the golangci invocation step. This will
# be included in the ci-export tar if the outbound tar stream succeeds.
echo "marker_before_golangci: $(date)" > "$OUTDIR/marker_before_golangci${SUFFIX}.txt" 2>/dev/null || true

# Ultra-aggressive cleanup (only when explicitly requested).
# This backs up the current GOROOT/pkg to /tmp, recreates an empty pkg tree
# restoring only 'tool' and 'include' so the 'go' binary remains usable, then
# attempts to rebuild stdlib. If the rebuild leaves 'go' broken we restore
# the backup to keep the container usable and record diagnostics.
if echo "${SUFFIX}" | grep -q "_blocking_force_ultra" || [ "${ULTRA_AGGRESSIVE_CLEAN:-0}" = "1" ]; then
  echo "ULTRA_AGGRESSIVE_CLEAN: backing up /usr/local/go/pkg -> /tmp/goroot_pkg_backup and recreating pkg tree" > "$OUTDIR/ultra${SUFFIX}.txt" 2>/dev/null || true
  if [ -d /usr/local.go/pkg ]; then
    rm -rf /tmp/goroot_pkg_backup || true
    # Move the pkg tree out-of-the-way so we start with a clean slate.
    mv /usr/local/go/pkg /tmp/goroot_pkg_backup || true
    mkdir -p /usr/local/go/pkg || true
    # Restore essential toolchain helpers so 'go' stays functional.
    if [ -d /tmp/goroot_pkg_backup/tool ]; then
      cp -a /tmp/goroot_pkg_backup/tool /usr/local/go/pkg/ || true
    fi
    if [ -d /tmp/goroot_pkg_backup/include ]; then
      cp -a /tmp/goroot_pkg_backup/include /usr/local/go/pkg/ || true
    fi
  fi

  # Attempt to rebuild the stdlib; prefer 'go install std' but fall back to 'go test std'.
  if /usr/local/go/bin/go install std >/tmp/gobuild_std_ultra.log 2>&1; then
    echo "ULTRA: go install std succeeded" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  else
    echo "ULTRA: go install std failed; attempting go test std (may take a while)" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
    if command -v timeout >/dev/null 2>&1; then
      timeout 300s /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
    else
      /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
    fi
  fi

  # Sanity-check that 'go' still works; restore backup if it doesn't.
  if /usr/local/go/bin/go version >/dev/null 2>&1; then
    echo "ULTRA: go binary functional after rebuild" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  else
    echo "ULTRA: go binary broken after aggressive cleanup; restoring backup" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
    rm -rf /usr/local/go/pkg || true
    mv /tmp/goroot_pkg_backup /usr/local/go/pkg || true
    echo "restored /usr/local/go/pkg from backup" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  fi
  # Ensure module cache populated after aggressive cleanup
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download || true
fi

## Install and run golangci-lint using the container's Go toolchain.
## Building golangci-lint inside the image ensures the export-data
## formats used by the tool match the container's compiler (fixes
## "unsupported version" import errors).
rc=0
echo "golangci_install: checking for golangci-lint in PATH" > "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
# If FORCE_REBUILD_GOLANGCI=1 is set, always rebuild golangci-lint with the
# container's Go toolchain into /go/bin. This is useful for quick experiments
# to ensure golangci was compiled with the exact container Go and caches.
if [ "${FORCE_REBUILD_GOLANGCI:-0}" = "1" ]; then
  echo "FORCE_REBUILD_GOLANGCI=1: forcing rebuild of golangci-lint into /go/bin" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  GOBIN=/go/bin /usr/local/go/bin/go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0 2>> "$OUTDIR/golangci_install_log${SUFFIX}.txt" || rc=$?
  GOLANGCI_BIN=/go/bin/golangci-lint
else
  # If golangci-lint is already present in PATH (for example, baked into the
  # analyzer image), prefer it and skip rebuilding. Otherwise install into
  # /go/bin (image-built location) using the container's go toolchain.
  if command -v golangci-lint >/dev/null 2>&1; then
    GOLANGCI_BIN=$(command -v golangci-lint)
    echo "found golangci-lint at ${GOLANGCI_BIN}" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  else
    echo "Installing golangci-lint v1.59.0 with container go" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
    GOBIN=/go/bin /usr/local/go/bin/go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0 2>> "$OUTDIR/golangci_install_log${SUFFIX}.txt" || rc=$?
    GOLANGCI_BIN=/go/bin/golangci-lint
  fi
fi
# Record go/golangci version info for debugging
/usr/local/go/bin/go version >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
if [ -n "${GOLANGCI_BIN:-}" ] && [ -x "$GOLANGCI_BIN" ]; then
  "$GOLANGCI_BIN" --version >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  # CI debug: also write which and version into the main export dir for quick inspection
  command -v golangci-lint > "$OUTDIR/which_golangci${SUFFIX}.txt" 2>&1 || true
  "$GOLANGCI_BIN" --version > "$OUTDIR/golangci_version${SUFFIX}.txt" 2>&1 || true

  # Rebuild the Go standard library so compiled export data under GOROOT
  # matches the container's toolchain. This helps avoid "unsupported
  # version" errors when golangci's typecheck imports stdlib packages.
  echo "Rebuilding Go stdlib (go install std) for consistent export-data" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  if /usr/local/go/bin/go install std > /tmp/gobuild_std_${SUFFIX}.log 2>&1; then
    echo "Rebuilt stdlib: success" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
    tail -n 200 /tmp/gobuild_std_${SUFFIX}.log >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  else
    echo "Rebuilt stdlib: failed (see /tmp/gobuild_std_${SUFFIX}.log)" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
    tail -n 200 /tmp/gobuild_std_${SUFFIX}.log >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  fi

    # Diagnostic: record GOROOT/GOOS/GOARCH and list compiled stdlib packages so
  # we can confirm export-data was rebuilt inside the container. This helps
  # diagnose "unsupported version" import errors by showing timestamps,
  # sizes, and presence of arch-specific package object files.
  /usr/local/go/bin/go env GOROOT GOOS GOARCH > "$OUTDIR/go_env_goroot_arch${SUFFIX}.txt" 2>&1 || true
  GOROOT_DIR=$(/usr/local/go/bin/go env GOROOT 2>/dev/null || echo "/usr/local/go")
  GOOS_VAL=$(/usr/local/go/bin/go env GOOS 2>/dev/null || echo "linux")
  GOARCH_VAL=$(/usr/local/go/bin/go env GOARCH 2>/dev/null || echo "amd64")
  # top-level pkg listing
  ls -la "$GOROOT_DIR/pkg" > "$OUTDIR/goroot_pkg_listing${SUFFIX}.txt" 2>&1 || true
  du -sh "$GOROOT_DIR/pkg" > "$OUTDIR/goroot_pkg_du${SUFFIX}.txt" 2>&1 || true
  # arch-specific directory listing (e.g. linux_amd64)
  ls -la "$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}" > "$OUTDIR/goroot_pkg_arch_listing${SUFFIX}.txt" 2>&1 || true
  # Show a sample of the arch dir (first 200 lines) to avoid huge files
  if [ -d "$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}" ]; then
    ls -la "$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}" | sed -n '1,200p' > "$OUTDIR/goroot_pkg_arch_sample${SUFFIX}.txt" 2>&1 || true
  fi

  # Targeted diagnostics for problematic stdlib package(s)
  # - stat matching files under the arch dir (timestamps, sizes, owner)
  # - hexdump the first 256 bytes of sync/atomic archive if present
  ARCH_DIR="$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}"
  echo "Collecting targeted sync/atomic diagnostics from $ARCH_DIR" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  for f in "$ARCH_DIR"/sync* "$ARCH_DIR"/*/sync*; do
    if [ -e "$f" ]; then
      stat -c '%n %s %Y %U %G' "$f" >> "$OUTDIR/goroot_pkg_sync_stat${SUFFIX}.txt" 2>&1 || true
    fi
  done
  # hexdump the sync/atomic .a if present
  if [ -f "$ARCH_DIR/sync/atomic.a" ]; then
    # prefer hexdump/xxd but fall back to od if not available in the analyzer image
    if command -v hexdump >/dev/null 2>&1; then
      hexdump -n 256 -C "$ARCH_DIR/sync/atomic.a" > "$OUTDIR/sync_atomic_hexdump${SUFFIX}.txt" 2>&1 || true
    elif command -v xxd >/dev/null 2>&1; then
      xxd -l 256 "$ARCH_DIR/sync/atomic.a" > "$OUTDIR/sync_atomic_hexdump${SUFFIX}.txt" 2>&1 || true
    elif command -v od >/dev/null 2>&1; then
      # od prints bytes in hex; mimic a short hexdump for triage
      od -An -v -tx1 -N256 "$ARCH_DIR/sync/atomic.a" | sed -E 's/^\s*//' > "$OUTDIR/sync_atomic_hexdump${SUFFIX}.txt" 2>&1 || true
    else
      echo "no hexdump/xxd/od available to dump $ARCH_DIR/sync/atomic.a" > "$OUTDIR/sync_atomic_hexdump${SUFFIX}.txt" 2>&1 || true
    fi
  fi

  # Module and cache diagnostics: list module cache root and find any compiled .a files
  ls -la /tmp/gomodcache > "$OUTDIR/gomodcache_ls${SUFFIX}.txt" 2>&1 || true
  find /tmp/gomodcache -type f -name '*.a' -print > "$OUTDIR/gomodcache_a_files${SUFFIX}.txt" 2>&1 || true
  firstmoda=$(find /tmp/gomodcache -type f -name '*sync*atomic*.a' -print -quit 2>/dev/null || true)
  if [ -n "$firstmoda" ]; then
    hexdump -n 256 -C "$firstmoda" > "$OUTDIR/gomodcache_sample_hexdump${SUFFIX}.txt" 2>&1 || true
  fi

  # Ensure diagnostics are copied into the job-bound CI export dir so the
  # workflow artifact uploader picks them up reliably. We copy (not move)
  # from the helper's local OUTDIR into CI_EXPORT_DIR and emit a small
  # manifest listing the files and sizes.
  mkdir -p "${CI_EXPORT_DIR}" 2>/dev/null || true
  files_to_copy=(
    "sync_atomic_preflight${SUFFIX}.json"
    "sync_atomic_preflight_status${SUFFIX}.txt"
    "go_env_goroot_arch${SUFFIX}.txt"
    "goroot_pkg_listing${SUFFIX}.txt"
    "goroot_pkg_du${SUFFIX}.txt"
    "goroot_pkg_arch_listing${SUFFIX}.txt"
    "goroot_pkg_arch_sample${SUFFIX}.txt"
    "goroot_pkg_sync_stat${SUFFIX}.txt"
    "sync_atomic_hexdump${SUFFIX}.txt"
    "gomodcache_ls${SUFFIX}.txt"
    "gomodcache_a_files${SUFFIX}.txt"
    "gomodcache_sample_hexdump${SUFFIX}.txt"
    "golangci_install_log${SUFFIX}.txt"
    "golangci_typecheck${SUFFIX}.out"
  )
  for fname in "${files_to_copy[@]}"; do
    if [ -e "$OUTDIR/$fname" ]; then
      cp -a "$OUTDIR/$fname" "$CI_EXPORT_DIR/" 2>/dev/null || true
    fi
    # also copy legacy/unsuffixed names so older post-mortem tooling can find them
    legacy="${fname%${SUFFIX}.txt}.txt"
    legacy_json="${fname%${SUFFIX}.json}.json"
    if [ -e "$OUTDIR/$legacy" ]; then
      cp -a "$OUTDIR/$legacy" "$CI_EXPORT_DIR/" 2>/dev/null || true
    fi
    if [ -e "$OUTDIR/$legacy_json" ]; then
      cp -a "$OUTDIR/$legacy_json" "$CI_EXPORT_DIR/" 2>/dev/null || true
    fi
  done
  # Always write a manifest so the artifact summary shows these files exist.
  (cd "$CI_EXPORT_DIR" 2>/dev/null && ls -la > "files_manifest${SUFFIX}.txt" 2>/dev/null) || true

  # Run golangci-lint (typecheck) after ensuring stdlib export-data matches.
  "$GOLANGCI_BIN" run --config .golangci.yml --enable typecheck ./... > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || rc=$?
else
  echo "golangci-lint not found or not executable: ${GOLANGCI_BIN:-<none>}" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  rc=2
fi
# Marker indicating golangci completed (successfully or not).
echo "marker_after_golangci: $(date) rc=${rc:-}" > "$OUTDIR/marker_after_golangci${SUFFIX}.txt" 2>/dev/null || true
echo "$rc" > "$OUTDIR/golangci_exit_code${SUFFIX}" || true
# final checkpoint so we know the container finished running the helper
echo "container_finished: $(date) rc=$rc" > "$OUTDIR/container_finished${SUFFIX}.txt" 2>/dev/null || true
exit "$rc"

if [ -x /usr/local/go/bin/go ]; then
  echo "Running conservative post-rebuild diagnostics" > "$OUTDIR/post_rebuild_diag${SUFFIX}.txt" 2>/dev/null || true
  # Ensure module files available
  # `go mod tidy` accepts no package arguments; run without './...' so it
  # operates on the repository's modules and writes output to diagnostics.
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod tidy > "$OUTDIR/go_mod_tidy${SUFFIX}.txt" 2>&1 || true
  # list module cache
  ls -la /tmp/gomodcache > "$OUTDIR/gomodcache_ls${SUFFIX}.txt" 2>/dev/null || true
  # capture dependency graph after rebuild
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps_after${SUFFIX}.json" 2>&1 || true
  # capture package list for workspace
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -json ./... > "$OUTDIR/go_list${SUFFIX}.json" 2>&1 || true
  # list toolchain linux_amd64 tools
  ls -la /usr/local/go/pkg/tool/linux_amd64 > "$OUTDIR/goroot_tool_linux_list${SUFFIX}.txt" 2>/dev/null || true
fi

# Ultra-aggressive cleanup (only when explicitly requested).
# This backs up the current GOROOT/pkg to /tmp, recreates an empty pkg tree
# restoring only 'tool' and 'include' so the 'go' binary remains usable, then
# attempts to rebuild stdlib. If the rebuild leaves 'go' broken we restore
# the backup to keep the container usable and record diagnostics.
if echo "${SUFFIX}" | grep -q "_blocking_force_ultra" || [ "${ULTRA_AGGRESSIVE_CLEAN:-0}" = "1" ]; then
  echo "ULTRA_AGGRESSIVE_CLEAN: backing up /usr/local/go/pkg -> /tmp/goroot_pkg_backup and recreating pkg tree" > "$OUTDIR/ultra${SUFFIX}.txt" 2>/dev/null || true
  if [ -d /usr/local/go/pkg ]; then
    rm -rf /tmp/goroot_pkg_backup || true
    # Move the pkg tree out-of-the-way so we start with a clean slate.
    mv /usr/local/go/pkg /tmp/goroot_pkg_backup || true
    mkdir -p /usr/local/go/pkg || true
    # Restore essential toolchain helpers so 'go' stays functional.
    if [ -d /tmp/goroot_pkg_backup/tool ]; then
      cp -a /tmp/goroot_pkg_backup/tool /usr/local/go/pkg/ || true
    fi
    if [ -d /tmp/goroot_pkg_backup/include ]; then
      cp -a /tmp/goroot_pkg_backup/include /usr/local/go/pkg/ || true
    fi
  fi

  # Attempt to rebuild the stdlib; prefer 'go install std' but fall back to 'go test std'.
  if /usr/local/go/bin/go install std >/tmp/gobuild_std_ultra.log 2>&1; then
    echo "ULTRA: go install std succeeded" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  else
    echo "ULTRA: go install std failed; attempting go test std (may take a while)" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
    if command -v timeout >/dev/null 2>&1; then
      timeout 300s /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
    else
      /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
    fi
  fi

  # Sanity-check that 'go' still works; restore backup if it doesn't.
  if /usr/local/go/bin/go version >/dev/null 2>&1; then
    echo "ULTRA: go binary functional after rebuild" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  else
    echo "ULTRA: go binary broken after aggressive cleanup; restoring backup" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
    rm -rf /usr/local/go/pkg || true
    mv /tmp/goroot_pkg_backup /usr/local/go/pkg || true
    echo "restored /usr/local/go/pkg from backup" >> "$OUTDIR/ultra${SUFFIX}.txt" || true
  fi
  # Ensure module cache populated after aggressive cleanup
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download || true
fi

## Install and run golangci-lint using the container's Go toolchain.
## Building golangci-lint inside the image ensures the export-data
## formats used by the tool match the container's compiler (fixes
## "unsupported version" import errors).
rc=0
echo "golangci_install: checking for golangci-lint in PATH" > "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
# If golangci-lint is already present in PATH (for example, baked into the
# analyzer image), prefer it and skip rebuilding. Otherwise install into
# /go/bin (image-built location) using the container's go toolchain.
if command -v golangci-lint >/dev/null 2>&1; then
  GOLANGCI_BIN=$(command -v golangci-lint)
  echo "found golangci-lint at ${GOLANGCI_BIN}" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
else
  echo "Installing golangci-lint v1.59.0 with container go" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  GOBIN=/go/bin /usr/local/go/bin/go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0 2>> "$OUTDIR/golangci_install_log${SUFFIX}.txt" || rc=$?
  GOLANGCI_BIN=/go/bin/golangci-lint
fi
# Record go/golangci version info for debugging
/usr/local/go/bin/go version >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
if [ -n "${GOLANGCI_BIN:-}" ] && [ -x "$GOLANGCI_BIN" ]; then
  "$GOLANGCI_BIN" --version >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  # CI debug: also write which and version into the main export dir for quick inspection
  command -v golangci-lint > "$OUTDIR/which_golangci${SUFFIX}.txt" 2>&1 || true
  "$GOLANGCI_BIN" --version > "$OUTDIR/golangci_version${SUFFIX}.txt" 2>&1 || true
  "$GOLANGCI_BIN" run --config .golangci.yml --enable typecheck ./... > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || rc=$?
else
  echo "golangci-lint not found or not executable: ${GOLANGCI_BIN:-<none>}" >> "$OUTDIR/golangci_install_log${SUFFIX}.txt" 2>&1 || true
  rc=2
fi
# Marker indicating golangci completed (successfully or not).
echo "marker_after_golangci: $(date) rc=${rc:-}" > "$OUTDIR/marker_after_golangci${SUFFIX}.txt" 2>/dev/null || true
echo "$rc" > "$OUTDIR/golangci_exit_code${SUFFIX}" || true
# final checkpoint so we know the container finished running the helper
echo "container_finished: $(date) rc=$rc" > "$OUTDIR/container_finished${SUFFIX}.txt" 2>/dev/null || true
exit "$rc"

  # Recreate the module cache after rebuilding stdlib
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download ./... || true
  # Ultra-aggressive option: when specifically requested (suffix contains
  # '_blocking_force_ultra' or ULTRA_AGGRESSIVE_CLEAN=1), back up and remove
  # the entire GOROOT pkg tree (including 'tool' and 'include') to ensure
  # any stray compiled artifacts are removed. This is destructive but run
  # only in debug/forced runs. If removing breaks the 'go' binary we
  # restore the backup to avoid leaving the container in a broken state.
  if echo "${SUFFIX}" | grep -q "_blocking_force_ultra" || [ "${ULTRA_AGGRESSIVE_CLEAN:-0}" = "1" ]; then
    echo "ULTRA_AGGRESSIVE_CLEAN: backing up /usr/local/go/pkg -> /tmp/goroot_pkg_backup and removing pkg/*" || true
    if [ -d /usr/local/go/pkg ]; then
      rm -rf /tmp/goroot_pkg_backup || true
      mv /usr/local/go/pkg /tmp/goroot_pkg_backup || true
      mkdir -p /usr/local/go/pkg || true
      # Restore toolchain helpers into the fresh pkg tree so `go` remains
      # functional while compiled stdlib packages are removed. Copying
      # only 'tool' and 'include' preserves essential binaries/headers but
      # leaves compiled package objects out of the active GOROOT/pkg.
      if [ -d /tmp/goroot_pkg_backup/tool ]; then
        cp -a /tmp/goroot_pkg_backup/tool /usr/local/go/pkg/ || true
      fi
      if [ -d /tmp/goroot_pkg_backup/include ]; then
        cp -a /tmp/goroot_pkg_backup/include /usr/local/go/pkg/ || true
      fi
    fi

    # attempt to rebuild stdlib now that pkg is empty
    if /usr/local/go/bin/go install std >/tmp/gobuild_std_ultra.log 2>&1; then
      echo "ULTRA: go install std succeeded" || true
    else
      echo "ULTRA: go install std failed; attempting go test std (timeout 300s)" || true
      if command -v timeout >/dev/null 2>&1; then
        timeout 300s /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
      else
        /usr/local/go/bin/go test std >/tmp/gobuild_std_ultra_test.log 2>&1 || true
      fi
    fi

    # sanity-check that 'go' still works (run 'go version') and that GOROOT/pkg
    # now has compiled entries; if not, restore the backup to keep the container
    # usable and record the restore in the output dir.
    if /usr/local/go/bin/go version >/dev/null 2>&1; then
      echo "ULTRA: go binary functional after rebuild" || true
    else
      echo "ULTRA: go binary broken after aggressive cleanup; restoring backup" || true
      rm -rf /usr/local/go/pkg || true
      mv /tmp/goroot_pkg_backup /usr/local/go/pkg || true
      echo "restored /usr/local/go/pkg from backup" || true
    fi
    # ensure module cache populated
    GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download ./... || true
  fi

if [ -x /usr/local/go/bin/go ]; then
  echo "Running conservative post-rebuild diagnostics" > "$OUTDIR/post_rebuild_diag${SUFFIX}.txt" 2>/dev/null || true
  # Ensure module files available
  # `go mod tidy` accepts no package arguments; run without './...' so it
  # operates on the repository's modules and writes output to diagnostics.
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod tidy > "$OUTDIR/go_mod_tidy${SUFFIX}.txt" 2>&1 || true
  # list module cache
  ls -la /tmp/gomodcache > "$OUTDIR/gomodcache_ls${SUFFIX}.txt" 2>/dev/null || true
  # capture dependency graph after rebuild
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps_after${SUFFIX}.json" 2>&1 || true
  # capture package list for workspace
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -json ./... > "$OUTDIR/go_list${SUFFIX}.json" 2>&1 || true
  # list toolchain linux_amd64 tools
  ls -la /usr/local/go/pkg/tool/linux_amd64 > "$OUTDIR/goroot_tool_linux_list${SUFFIX}.txt" 2>/dev/null || true
fi