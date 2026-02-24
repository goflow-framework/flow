## Multi-stage Dockerfile for building a minimal Flow app

FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
WORKDIR /src/examples/docker_deploy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /app ./

FROM gcr.io/distroless/static-debian11
COPY --from=builder /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
