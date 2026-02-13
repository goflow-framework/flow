# Observability (metrics & tracing)

This document explains the built-in observability features in Flow and how to enable them for local development and testing.

Contents
- Prometheus metrics (HTTP handlers)
- Stdout tracer (quick local tracing)
- OTLP exporter (send traces to a Collector)
- Local docker-compose example: Collector + Prometheus + Grafana

## Prometheus metrics

Flow exposes request-level Prometheus metrics when you enable the metrics middleware and run a metrics HTTP endpoint.

From the CLI (development server):

- `--metrics-addr` — start a small HTTP server that serves `/metrics` on the provided address (for example `:9090`).

Example (run the example app and expose metrics on :9090):

```bash
flow serve --addr :3000 --metrics-addr :9090
```

When `--metrics-addr` is provided the CLI will also register the HTTP instrumentation middleware on the App, which records per-route request counts and latencies. The metrics names are prefixed with `flow_` (see generated metric names in the code: `flow_http_requests_total` and `flow_http_request_duration_seconds`).

Programmatic usage (within an app):

```go
app := flow.New("my-service",
    flow.WithAddr(":3000"),
    flow.WithPrometheus(), // registers middleware
)

// separately start a /metrics server using promhttp.Handler()
go http.ListenAndServe(":9090", promhttp.Handler())
```

Notes
- The metrics server is intentionally a separate HTTP listener so you can expose metrics on a different port or interface.
- You can scrape `/metrics` from Prometheus by adding a `scrape_config` job that points to the address you chose.

## Stdout tracer (quick local tracing)

The stdout tracer is a light-weight OpenTelemetry exporter that writes spans to stdout in a readable format. It's useful for local development and CI where you want a quick trace dump.

CLI flag:

- `--trace-stdout` — enable the stdout exporter for the running process.

Example:

```bash
flow serve --addr :3000 --trace-stdout --service-name my-service
```

Programmatic option (App construction):

```go
app := flow.New("my-service",
    flow.WithAddr(":3000"),
    flow.WithStdoutTracer("my-service", observability.StdoutTracerOptions{
        Sampler: "probabilistic", // "always", "never", or "probabilistic"
        Probability: 0.1,
        MaxExportBatchSize: 64,
    }),
)
```

The App will keep a shutdown function for the tracer and call it when `App.Shutdown(ctx)` runs, ensuring spans are flushed.

## OTLP exporter (Collector / remote backend)

For real tracing workflows you'll typically send spans to an OpenTelemetry Collector (OTel Collector) which can forward to backends like Jaeger, Tempo, Honeycomb, or Lightstep.

CLI flags (added to `cmd/flow` and `cmd/flow-worker`):

- `--otlp-endpoint` — OTLP gRPC endpoint (for example `otel-collector:4317`).
- `--otlp-insecure` — use insecure gRPC (no TLS) — useful for local Collector.
- `--otlp-headers` — comma-separated `key=val` headers to include with OTLP requests (useful for API keys): e.g. `api-key=foo,env=dev`.

Example (use the Collector at `otel-collector:4317`):

```bash
flow serve --addr :3000 --otlp-endpoint otel-collector:4317 --otlp-insecure --service-name my-service
```

Programmatic option (App construction):

```go
headers := map[string]string{"api-key": "secret"}
app := flow.New("my-service",
    flow.WithAddr(":3000"),
    flow.WithOTLPExporter("otel-collector:4317", "my-service", true, headers),
)
```

Behavior and precedence
- If `--otlp-endpoint` is provided, the CLI will wire the OTLP exporter into the App during construction (programmatic `WithOTLPExporter` does the same).
- If an OTLP endpoint is provided and you also pass `--trace-stdout`, both exporters can exist but typically you'll just use OTLP for production and stdout for local dev. The default CLI behavior prefers OTLP when `--otlp-endpoint` is set and will only enable stdout tracing when OTLP is not configured (to avoid duplicate exports by default).

## Local Docker Compose example (Collector + Prometheus + Grafana)

This compose file is suitable for local testing. It runs:
- OpenTelemetry Collector (receives OTLP/gRPC on 4317 and exposes metrics on 8888)
- Prometheus (scrapes Flow `/metrics` and Collector metrics)
- Grafana (visualize metrics)

Save the following as `dev/observability-compose.yml` in your project root and run `docker compose -f dev/observability-compose.yml up`.

```yaml
version: "3.8"
services:
  otel-collector:
    image: otel/opentelemetry-collector:0.74.0
    command: ["--config=/etc/otel-collector-config/config.yaml"]
    volumes:
      - ./dev/otel-collector-config:/etc/otel-collector-config
    ports:
      - "4317:4317"   # OTLP/gRPC
      - "8888:8888"   # Collector metrics (prometheus receiver/exporter)

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./dev/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    ports:
      - "3001:3000"

# Notes: create ./dev/otel-collector-config/config.yaml and ./dev/prometheus.yml as shown below
```

dev/otel-collector-config/config.yaml (minimal)

```yaml
receivers:
  otlp:
    protocols:
      grpc: {}

exporters:
  logging:
    logLevel: info

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging]

extensions: {}
```

dev/prometheus.yml (scrape Flow `/metrics` and Collector metrics)

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'flow'
    static_configs:
      - targets: ['host.docker.internal:9090'] # adjust to your host binding

  - job_name: 'otel-collector'
    static_configs:
      - targets: ['otel-collector:8888']
```

Notes
- On Linux you may need to replace `host.docker.internal` with your host IP or run the Flow app inside Docker so Prometheus can reach it.
- The Collector config above is intentionally minimal — you can extend it to forward traces to any backend (Jaeger, Tempo, Honeycomb, etc.) by adding appropriate exporters.

## Grafana

- Start Grafana (port 3001 in the compose above) and add a Prometheus data source pointed at `http://prometheus:9090` (when Grafana runs inside the same compose network you can use the service name). For a host-bound Grafana, point it at `http://localhost:9090`.
- Import panels or create your own dashboards. A simple dashboard can chart `flow_http_request_duration_seconds_bucket` (histogram buckets) or `rate(flow_http_requests_total[1m])`.

## Quick try-it checklist

1. Start local infra:

```bash
docker compose -f dev/observability-compose.yml up -d
```

2. Run the Flow example app with metrics and OTLP enabled (or only metrics):

```bash
flow serve --addr :3000 --metrics-addr :9090 --otlp-endpoint localhost:4317 --otlp-insecure --service-name my-service
```

3. Open Prometheus (http://localhost:9090) and query `flow_http_requests_total`.
4. Open Grafana (http://localhost:3001, user=admin pass=admin) and add Prometheus as a data source.

## Troubleshooting

- If you don't see metrics in Prometheus, verify the address you passed to `--metrics-addr` is reachable from the Prometheus container. On Linux you may need to bind to `0.0.0.0` and use the host IP in `prometheus.yml`.
- If OTLP exporter fails to connect, check Collector logs and ensure port 4317 is reachable. Use `--otlp-insecure` for local, non-TLS Collector setups.

## Next steps

- Add a small Grafana dashboard JSON to `dev/grafana` for quick import.
- Add a `--otlp-headers-file` flag to allow reading long secrets from a file or environment helper.

If you'd like, I can also create the `dev/` files in the repo (`observability-compose.yml`, Collector config, Prometheus config`) and a simple Grafana dashboard JSON next — tell me to proceed and I'll add them and run a quick `go build` to ensure no code changes are required.
