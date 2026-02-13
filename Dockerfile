## Multi-stage Dockerfile for Flow
## Build stage
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git build-base ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o /out/flow ./cmd/flow

## Final stage
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/flow /usr/local/bin/flow
USER nobody:nogroup
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/flow"]
