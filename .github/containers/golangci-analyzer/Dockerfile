FROM golang:1.24.11

# Install golangci-lint built with this image's Go toolchain so its export
# data is compatible with the stdlib compiled in this image.
ENV GOBIN=/usr/local/bin
ENV GOFLAGS='-mod=mod -buildvcs=false'

RUN set -eux; \
    # install golangci-lint (pinned)
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0; \
    # also install some helper tools commonly used by the repo's helper script
    GOBIN=/usr/local/bin go install golang.org/x/tools/cmd/goimports@latest || true; \
    GOBIN=/usr/local/bin go install github.com/kisielk/errcheck@latest || true;

WORKDIR /app

ENTRYPOINT ["/bin/sh", "-c"]
