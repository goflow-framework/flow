FROM golang:1.24.11

# Install golangci-lint built with this image's Go toolchain so its export
# data is compatible with the stdlib compiled in this image.
ENV GOBIN=/go/bin
ENV GOFLAGS='-mod=mod -buildvcs=false'

RUN set -eux; \
    # install golangci-lint (pinned)
    # ensure basic network tooling present so the helper script can curl/tar
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg2; \
    rm -rf /var/lib/apt/lists/*; \
    # install golangci-lint (pinned) into /go/bin
    GOBIN=/go/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0; \
    # also install some helper tools commonly used by the repo's helper script
    GOBIN=/go/bin go install golang.org/x/tools/cmd/goimports@latest || true; \
    GOBIN=/go/bin go install github.com/kisielk/errcheck@latest || true; \
    # Rebuild the standard library inside the image so GOROOT/pkg compiled
    # export-data matches the image's Go toolchain (avoids "unsupported
    # version" errors when analyzers import std packages).
    /usr/local/go/bin/go install std || true;

WORKDIR /app

ENTRYPOINT ["/bin/sh", "-c"]
