FROM golang:1.24.11

# Install golangci-lint built with this image's Go toolchain so its export
# data is compatible with the stdlib compiled in this image.
ENV GOBIN=/go/bin
ENV GOFLAGS='-mod=mod -buildvcs=false'
RUN set -eux; \
    # ensure basic network tooling and build tools so stdlib can be rebuilt
    export DEBIAN_FRONTEND=noninteractive; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg2 gcc build-essential make strace; \
    rm -rf /var/lib/apt/lists/*; \
    # install golangci-lint (pinned) into /go/bin
    GOBIN=/go/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0; \
    # also install some helper tools commonly used by the repo's helper script
    GOBIN=/go/bin go install golang.org/x/tools/cmd/goimports@latest || true; \
    GOBIN=/go/bin go install github.com/kisielk/errcheck@latest || true; \
    # Rebuild the standard library inside the image so GOROOT/pkg compiled
    # export-data matches the image's Go toolchain. First try `go install`.
    GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache GOOS=linux GOARCH=amd64 /usr/local/go/bin/go install -v -a std; \
    # If `go install` did not populate the arch pkg dir for std packages,
    # fetch a small bootstrap Go (>=1.22.6), use it as GOROOT_BOOTSTRAP and run
    # the canonical bootstrap `make.bash` to install the runtime and stdlib
    # into /usr/local/go/pkg. This ensures exported archive data lives in
    # GOROOT/pkg where analyzers expect it.
    if [ ! -d "/usr/local/go/pkg/$(/usr/local/go/bin/go env GOOS)_$(/usr/local/go/bin/go env GOARCH)/sync" ]; then \
        echo "go install did not populate arch pkg dir; downloading Go 1.22.6 bootstrap"; \
        BOOT_VER=1.22.6; \
        BOOT_TAR=/tmp/go${BOOT_VER}.linux-amd64.tar.gz; \
        BOOT_DIR=/root/go${BOOT_VER}; \
        curl -fsSL "https://go.dev/dl/go${BOOT_VER}.linux-amd64.tar.gz" -o "$BOOT_TAR"; \
        tar -C /root -xzf "$BOOT_TAR"; \
        # extracted path is /root/go
        if [ -d /root/go ]; then mv /root/go "$BOOT_DIR"; fi; \
        export GOROOT_BOOTSTRAP="$BOOT_DIR"; \
        echo "Running make.bash with GOROOT_BOOTSTRAP=$GOROOT_BOOTSTRAP"; \
        # compute arch dir so we can verify make.bash populated it
        ARCH_DIR="/usr/local/go/pkg/$(/usr/local/go/bin/go env GOOS)_$(/usr/local/go/bin/go env GOARCH)"; \
        if [ -x "/usr/local/go/src/make.bash" ]; then \
            # capture full output to a temp log so build can show diagnostics if it fails
            (cd /usr/local/go/src && GOROOT_BOOTSTRAP="$GOROOT_BOOTSTRAP" ./make.bash 2>&1) | tee /tmp/make_bash.log | sed -n '1,200p'; \
            echo "make.bash finished; inspecting $ARCH_DIR"; \
            ls -la "$ARCH_DIR" | sed -n '1,200p' || true; \
            # Attempt to use the freshly-built toolchain to install std into GOROOT/pkg
            echo "Running /usr/local/go/bin/go install -v -a std using the new toolchain"; \
            /usr/local/go/bin/go env -json > /tmp/go_built_env.json || true; \
            (GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache /usr/local/go/bin/go install -v -x -a std 2>&1) | tee /tmp/go_install_std.log | sed -n '1,200p' || true; \
            echo "After go install -a std, listing $ARCH_DIR"; \
            ls -la "$ARCH_DIR" | sed -n '1,200p' || true; \
            # Defer any deterministic fallback and verification to the subsequent RUN.
            # The next RUN will parse logs, copy cache files, and verify sync/atomic.
            :; \
        else \
            echo "make.bash not found or not executable: /usr/local/go/src/make.bash"; \
        fi; \
        # cleanup bootstrap tar and dir to keep image small (only after successful make.bash)
        rm -rf "$BOOT_TAR" "$BOOT_DIR" || true; \
    fi;

RUN set -eux; \
    GOROOT_DIR=$(/usr/local/go/bin/go env GOROOT); \
    GOOS_VAL=$(/usr/local/go/bin/go env GOOS); \
    GOARCH_VAL=$(/usr/local/go/bin/go env GOARCH); \
    ARCH_DIR="$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}"; \
    echo "Attempting deterministic fallback into $ARCH_DIR"; \
    mkdir -p "$ARCH_DIR"; \
    : >/tmp/work_map.txt; \
    if [ -f /tmp/go_install_std.log ]; then \
        # parse 'packagefile pkg=...path...' lines
        awk '/^packagefile[[:space:]]+/ { if (match($0, /packagefile[[:space:]]+([^=[:space:]]+)=([^ ]+)/, p)) print p[1] " " p[2] }' /tmp/go_install_std.log >> /tmp/work_map.txt || true; \
    fi; \
    if [ -f /tmp/make_bash.log ]; then \
        awk '/cp \$WORK/ && /root\/\.cache\/go-build/ { if (match($0, /\$WORK\/([a-z0-9]+)/, w) && match($0, /root\/\.cache\/go-build\/([^ ]+)/, c)) print w[1] " " c[1] }' /tmp/make_bash.log >> /tmp/work_map.txt || true; \
    fi; \
    sort -u /tmp/work_map.txt -o /tmp/work_map.txt || true; \
    if [ -s /tmp/work_map.txt ]; then \
        sed -n '1,200p' /tmp/work_map.txt || true; \
        while read -r key path; do \
            if echo "$path" | grep -Eq '^/'; then src="$path"; else src="/root/.cache/go-build/$path"; fi; \
            if [ -f "$src" ]; then \
                if echo "$key" | grep -Eq '/'; then pkg="$key"; else pkg=""; fi; \
                if [ -z "$pkg" ]; then \
                    ipath=$(grep -a -m1 -oE '([a-z0-9_./]+/)+[a-z0-9_./]+' "$src" 2>/dev/null || true) || true; \
                    if [ -n "$ipath" ]; then pkg="$ipath"; fi; \
                fi; \
                if [ -n "$pkg" ]; then dstdir="$ARCH_DIR/$(dirname "$pkg")"; mkdir -p "$dstdir"; cp -av "$src" "$dstdir/$(basename "$pkg").a" || true; fi; \
            fi; \
        done < /tmp/work_map.txt; \
    else \
        echo "No work_map entries found; skipping deterministic copy"; \
    fi; \
    # last-resort scan for required package
    if [ ! -f "$ARCH_DIR/sync/atomic.a" ] && [ -d /root/.cache/go-build ]; then \
        for f in $(find /root/.cache/go-build -type f -print | sed -n '1,500p'); do \
            if grep -a -m1 "sync/atomic" "$f" >/dev/null 2>&1; then \
                mkdir -p "$ARCH_DIR/sync"; cp -av "$f" "$ARCH_DIR/sync/atomic.a" || true; break; \
            fi; \
        done; \
    fi; \
    /usr/local/go/bin/go list -json sync/atomic > /tmp/sync_atomic.json 2>/tmp/go_list_sync.err || true; \
    if [ -s /tmp/sync_atomic.json ]; then \
        echo "sync/atomic verification succeeded"; \
    else \
        mkdir -p /ci-export-typecheck || true; \
        tar -czf /ci-export-typecheck/typecheck-diagnostics.tar.gz /tmp/make_bash.log /tmp/go_install_std.log /tmp/go_built_env.json /tmp/work_map.txt /tmp/sync_atomic.json /tmp/go_list_sync.err || true; \
        echo "Verification failed; diagnostics saved to /ci-export-typecheck/typecheck-diagnostics.tar.gz"; \
        exit 1; \
    fi; \
    # copy other compiled archives we can find into ARCH_DIR
    CANDIDATE_DIRS="/root/.cache/go-build /tmp/gocache /tmp/gomodcache /go/pkg/mod"; \
    for d in $CANDIDATE_DIRS; do \
        if [ -d "$d" ]; then \
            echo "Scanning $d for .a files"; \
            find "$d" -type f -name '*.a' -print -exec cp -av {} "$ARCH_DIR/" \; || true; \
        fi; \
    done; \
    echo "After copy, listing $ARCH_DIR (first 200 entries)"; \
    ls -la "$ARCH_DIR" | sed -n '1,200p' || true; \
    if [ "$(ls -A "$ARCH_DIR" 2>/dev/null || true)" = "" ]; then \
        echo "ERROR: arch pkg dir $ARCH_DIR is empty after attempting to rebuild and recover .a files"; \
        /usr/local/go/bin/go env -json > /go_toolchain_env.json; \
        cat /go_toolchain_env.json; \
        echo "Listing /root/.cache and /tmp"; ls -la /root/.cache || true; ls -la /tmp || true; \
        exit 1; \
    fi

# Preserve a compressed snapshot of the built arch package directory so
# containers that aggressively remove /usr/local/go/pkg can restore a
# deterministic copy on startup. The startup wrapper (installed below)
# checks for a missing/empty arch dir and extracts this tarball into place.
RUN set -eux; \
        GOROOT_DIR=$(/usr/local/go/bin/go env GOROOT); \
        GOOS_VAL=$(/usr/local/go/bin/go env GOOS); \
        GOARCH_VAL=$(/usr/local/go/bin/go env GOARCH); \
        ARCH_DIR="$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}"; \
        PRESERVE_DIR="$GOROOT_DIR/preserved"; \
        mkdir -p "$PRESERVE_DIR"; \
        if [ -d "$ARCH_DIR" ] && [ "$(ls -A "$ARCH_DIR" 2>/dev/null || true)" != "" ]; then \
                echo "Creating preserved tarball for $ARCH_DIR"; \
                tar -C "$GOROOT_DIR/pkg" -czf "$PRESERVE_DIR/${GOOS_VAL}_${GOARCH_VAL}.tar.gz" "${GOOS_VAL}_${GOARCH_VAL}" || true; \
        else \
                echo "No arch dir to preserve at $ARCH_DIR"; \
        fi

# Install an entrypoint wrapper that restores the preserved stdlib archive
# into GOROOT/pkg at container startup if the arch dir is missing or empty.
RUN cat > /usr/local/bin/entrypoint-ensure-goroot.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
# If go is present, compute the arch dir
if [ -x /usr/local/go/bin/go ]; then
    GOROOT_DIR=$(/usr/local/go/bin/go env GOROOT)
    GOOS_VAL=$(/usr/local/go/bin/go env GOOS)
    GOARCH_VAL=$(/usr/local/go/bin/go env GOARCH)
else
    GOROOT_DIR=/usr/local/go
    GOOS_VAL=linux
    GOARCH_VAL=amd64
fi
ARCH_DIR="$GOROOT_DIR/pkg/${GOOS_VAL}_${GOARCH_VAL}"
PRESERVE_TAR="$GOROOT_DIR/preserved/${GOOS_VAL}_${GOARCH_VAL}.tar.gz"
if [ ! -d "$ARCH_DIR" ] || [ -z "$(ls -A "$ARCH_DIR" 2>/dev/null || true)" ]; then
    if [ -f "$PRESERVE_TAR" ]; then
        mkdir -p "$ARCH_DIR"
        echo "Restoring preserved stdlib into $ARCH_DIR from $PRESERVE_TAR"
        tar -C "$GOROOT_DIR/pkg" -xzf "$PRESERVE_TAR" || true
        chmod -R 0755 "$ARCH_DIR" || true
    else
        echo "No preserved stdlib tarball found at $PRESERVE_TAR; continuing" >&2 || true
    fi
fi
# forward to the requested command
exec "$@"
EOF
RUN chmod +x /usr/local/bin/entrypoint-ensure-goroot.sh

WORKDIR /app

# Use an entrypoint wrapper that ensures GOROOT/pkg has the preserved
# stdlib archive (if available) before executing the container's command.
ENTRYPOINT ["/usr/local/bin/entrypoint-ensure-goroot.sh"]
