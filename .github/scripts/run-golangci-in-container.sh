#!/bin/sh
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
OUTDIR="./ci-export-typecheck"
GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v1.59.0/golangci-lint-1.59.0-linux-amd64.tar.gz"

mkdir -p "$OUTDIR" /tmp/gomodcache /tmp/gocache
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

# Aggressive-clean path (for debugging): when running the forced blocking
# debug job we may need to aggressively clear caches and rebuild the stdlib
# export data inside the container to remove any build-id / export-data
# mismatches. This block runs when SUFFIX contains '_blocking_force' or when
# the environment variable AGGRESSIVE_CLEAN is set to '1'. It attempts a
# conservative cache wipe, then tries to rebuild the stdlib export data by
# running `go install std` (fallback to `go test std` if needed).
if echo "${SUFFIX}" | grep -q "_blocking_force" || [ "${AGGRESSIVE_CLEAN:-0}" = "1" ]; then
  echo "AGGRESSIVE_CLEAN: clearing module and go caches and attempting to rebuild stdlib export data" || true
  rm -rf /tmp/gocache/* /tmp/gomodcache/* || true
  # remove any remaining compiled stdlib packages (preserve tool/include as before)
  if [ -d /usr/local/go/pkg ]; then
    for entry in /usr/local/go/pkg/*; do
      base="$(basename "$entry")"
      if [ "$base" = "tool" ] || [ "$base" = "include" ]; then
        continue
      fi
      rm -rf "$entry" || true
    done
  fi
  # Attempt to rebuild standard library export data by installing std. This
  # compiles std packages with the container's go toolchain. If install
  # isn't supported, fall back to running `go test std` (capped with timeout).
  if /usr/local/go/bin/go install std >/tmp/gobuild_std.log 2>&1; then
    echo "go install std succeeded" || true
  else
    echo "go install std failed; attempting go test std (timeout 300s)" || true
    # run tests to force compilation of std packages; limit runtime to 5m
    if command -v timeout >/dev/null 2>&1; then
      timeout 300s /usr/local/go/bin/go test std >/tmp/gobuild_std_test.log 2>&1 || true
    else
      /usr/local/go/bin/go test std >/tmp/gobuild_std_test.log 2>&1 || true
    fi
  fi
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
fi

# Capture GOROOT pkg layout after cleanup
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_after${SUFFIX}.txt" 2>/dev/null || true

# Conservative fix: ensure module files are present and capture extra diagnostics
# after any rebuild/cleanup attempt. This helps diagnose "no go files to analyze"
# and shows the module graph and cached modules available to the container.
if [ -x /usr/local/go/bin/go ]; then
  echo "Running conservative post-rebuild diagnostics" > "$OUTDIR/post_rebuild_diag${SUFFIX}.txt" 2>/dev/null || true
  # Ensure module files available
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod tidy ./... > "$OUTDIR/go_mod_tidy${SUFFIX}.txt" 2>&1 || true
  # list module cache
  ls -la /tmp/gomodcache > "$OUTDIR/gomodcache_ls${SUFFIX}.txt" 2>/dev/null || true
  # capture dependency graph after rebuild
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps_after${SUFFIX}.json" 2>&1 || true
  # capture package list for workspace
  GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go list -json ./... > "$OUTDIR/go_list${SUFFIX}.json" 2>&1 || true
  # list toolchain linux_amd64 tools
  ls -la /usr/local/go/pkg/tool/linux_amd64 > "$OUTDIR/goroot_tool_linux_list${SUFFIX}.txt" 2>/dev/null || true
fi

# Collect diagnostics
echo "$PATH" > "$OUTDIR/path${SUFFIX}.txt" 2>/dev/null || true
which go > "$OUTDIR/which_go${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go version > "$OUTDIR/go_version${SUFFIX}.txt" 2>&1 || true
/usr/local/go/bin/go env -json > "$OUTDIR/go_env${SUFFIX}.json" 2>&1 || true
/usr/local/go/bin/go list -deps -json ./... > "$OUTDIR/deps${SUFFIX}.json" 2>&1 || true
/usr/local/go/bin/go list -json sync/atomic > "$OUTDIR/sync_atomic${SUFFIX}.json" 2>&1 || true
ls -la /usr/local/go/bin > "$OUTDIR/goroot_bin_ls${SUFFIX}.txt" 2>&1 || true
ls -la /usr/local/go/pkg > "$OUTDIR/goroot_pkg_ls${SUFFIX}.txt" 2>&1 || true

## Build/install golangci-lint inside the container so the binary is built
## with the same Go toolchain as the container's GOROOT. This avoids
## export-data version mismatches that happen when a prebuilt binary is
## produced with a different Go version than the container's stdlib.
rc=0
GOBIN=/tmp/gobin
mkdir -p "$GOBIN"
export GOBIN
export PATH="$GOBIN:$PATH"

# Try to install golangci-lint via `go install`. This compiles the tool
# with the container's go and places it in /tmp/gobin. If this fails we
# fall back to the prebuilt tarball download (older behavior).
if /usr/local/go/bin/go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0 >/dev/null 2>&1; then
  echo "golangci-lint installed to $GOBIN" >/dev/null 2>&1 || true
else
  # fallback: try to download the prebuilt archive into /tmp
  curl -sSfL "$GOLANGCI_URL" | tar -xz -C /tmp || rc=2
fi

# Ensure module deps are available in the container module cache
GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache /usr/local/go/bin/go mod download ./... || true

# Run golangci-lint (installed in GOBIN or extracted into /tmp)
if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run --config .golangci.yml --enable typecheck ./... > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || rc=$?
elif [ -x /tmp/golangci-lint-1.59.0-linux-amd64/golangci-lint ]; then
  /tmp/golangci-lint-1.59.0-linux-amd64/golangci-lint run --config .golangci.yml --enable typecheck ./... > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || rc=$?
else
  echo "no golangci-lint available" > "$OUTDIR/golangci_typecheck${SUFFIX}.out" 2>&1 || true
  rc=3
fi

# Copy any ultra/aggressive-clean logs and backup listings into the output
# directory so they are preserved in artifacts for debugging.
for f in /tmp/gobuild_std_ultra.log /tmp/gobuild_std_ultra_test.log /tmp/gobuild_std.log /tmp/gobuild_std_test.log; do
  if [ -f "$f" ]; then
    cp "$f" "$OUTDIR/$(basename "$f")${SUFFIX}" 2>/dev/null || true
  fi
done

if [ -d /tmp/goroot_pkg_backup ]; then
  ls -la /tmp/goroot_pkg_backup > "$OUTDIR/goroot_pkg_backup_ls${SUFFIX}.txt" 2>/dev/null || true
  find /tmp/goroot_pkg_backup -maxdepth 4 -type f | head -n 200 > "$OUTDIR/goroot_pkg_backup_files${SUFFIX}.txt" 2>/dev/null || true
fi

echo "$rc" > "$OUTDIR/golangci_exit_code${SUFFIX}" || true
exit "$rc"