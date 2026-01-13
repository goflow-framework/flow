package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/undiegomejia/flow/pkg/job"
	"github.com/undiegomejia/flow/pkg/observability"
)

func main() {
	// CLI flags
	addr := flag.String("redis", "localhost:6379", "redis address")
	ns := flag.String("ns", "flowjobs", "redis namespace prefix")
	handlersFlag := flag.String("handlers", "example", "comma-separated list of handler names to register (available: example,sleep,echo)")
	concurrency := flag.Int("concurrency", 1, "number of concurrent job handlers")
	pollMs := flag.Int("poll-ms", 250, "poll interval in milliseconds")
	backoffMs := flag.Int("backoff-ms", 500, "base backoff in milliseconds")
	jitter := flag.Int("jitter-ms", 50, "max jitter in milliseconds")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	metricsAddr := flag.String("metrics-addr", "", "optional address to expose Prometheus /metrics (eg. :9090)")
	traceStdout := flag.Bool("trace-stdout", false, "enable stdout OpenTelemetry tracer for local debugging")
	serviceName := flag.String("service-name", "flow-worker", "service.name used by tracing exporter")
	otlpEndpoint := flag.String("otlp-endpoint", "", "OTLP gRPC endpoint (e.g. otel-collector:4317)")
	otlpInsecure := flag.Bool("otlp-insecure", false, "use insecure gRPC connection for OTLP (local collector)")
	otlpHeaders := flag.String("otlp-headers", "", "comma-separated key=val headers for OTLP (e.g. api-key=foo)")
	flag.Parse()

	// logger
	logger := log.New(os.Stdout, "flow-worker: ", log.LstdFlags)

	opts := &redis.Options{Addr: *addr}
	q := job.NewRedisQueue(opts, *ns)
	defer q.Close()

	// optionally expose metrics endpoint
	if *metricsAddr != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(*metricsAddr, mux); err != nil {
				logger.Printf("metrics server error: %v", err)
			}
		}()
	}

	// Tracing: prefer OTLP exporter when endpoint provided, otherwise fall back to stdout tracer.
	var tracerShutdown func(context.Context) error
	if *otlpEndpoint != "" {
		headersMap := observability.ParseHeaders(*otlpHeaders)
		shutdown, err := observability.SetupOTLPTracer(context.Background(), *otlpEndpoint, *otlpInsecure, headersMap, *serviceName)
		if err != nil {
			logger.Printf("failed to setup OTLP tracer: %v", err)
		} else {
			tracerShutdown = shutdown
			defer func() { _ = tracerShutdown(context.Background()) }()
		}
	} else if *traceStdout {
		shutdown, err := observability.SetupStdoutTracer(*serviceName, observability.StdoutTracerOptions{})
		if err != nil {
			logger.Printf("failed to setup tracer: %v", err)
		} else {
			tracerShutdown = shutdown
			defer func() { _ = tracerShutdown(context.Background()) }()
		}
	}

	// register selectable handlers
	handlers := make(map[string]job.Handler)
	for _, h := range strings.Split(*handlersFlag, ",") {
		switch strings.TrimSpace(h) {
		case "example":
			handlers["example"] = wrapHandler(logger, *verbose, exampleHandler)
		case "sleep":
			handlers["sleep"] = wrapHandler(logger, *verbose, sleepHandler)
		case "echo":
			handlers["echo"] = wrapHandler(logger, *verbose, echoHandler)
		default:
			logger.Printf("unknown handler %q, skipping", h)
		}
	}

	wopts := job.WorkerOptions{
		PollInterval: time.Duration(*pollMs) * time.Millisecond,
		BackoffBase:  time.Duration(*backoffMs) * time.Millisecond,
		JitterMillis: *jitter,
		Concurrency:  *concurrency,
	}

	w := job.NewWorker(q, handlers, wopts)

	// graceful shutdown: cancel context on signal, call Stop and wait for Start to return
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- w.Start(ctx)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		logger.Printf("received signal %v, shutting down", s)
		cancel()
		// also signal worker to stop ASAP
		w.Stop()
		// wait for Start to return
		if err := <-done; err != nil {
			logger.Printf("worker stopped with error: %v", err)
		} else {
			logger.Printf("worker stopped")
		}
	case err := <-done:
		if err != nil {
			logger.Printf("worker exited: %v", err)
			os.Exit(1)
		}
		logger.Printf("worker exited cleanly")
	}
}

// wrapHandler adds logging around handlers when verbose is enabled.
func wrapHandler(logger *log.Logger, verbose bool, h job.Handler) job.Handler {
	if !verbose {
		return h
	}
	return func(ctx context.Context, j *job.Job) error {
		logger.Printf("start job id=%s type=%s attempts=%d", j.ID, j.Type, j.Attempts)
		err := h(ctx, j)
		if err != nil {
			logger.Printf("job id=%s failed: %v", j.ID, err)
		} else {
			logger.Printf("job id=%s completed", j.ID)
		}
		return err
	}
}

// exampleHandler demonstrates a trivial job.
func exampleHandler(ctx context.Context, j *job.Job) error {
	fmt.Printf("processing example job: id=%s type=%s\n", j.ID, j.Type)
	return nil
}

// sleepHandler sleeps for a few milliseconds if payload contains a duration (ms).
func sleepHandler(ctx context.Context, j *job.Job) error {
	// payload expected: {"ms":100}
	var p struct {
		Ms int `json:"ms"`
	}
	if len(j.Payload) > 0 {
		_ = jsonUnmarshal(j.Payload, &p)
	}
	d := time.Duration(p.Ms) * time.Millisecond
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	select {
	case <-time.After(d):
		fmt.Printf("slept %v for job %s\n", d, j.ID)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// echoHandler prints the job payload.
func echoHandler(ctx context.Context, j *job.Job) error {
	fmt.Printf("echo job %s payload=%s\n", j.ID, string(j.Payload))
	return nil
}

// minimal JSON unmarshal helper to avoid importing encoding/json repeatedly in this file
func jsonUnmarshal(b []byte, v interface{}) error {
	return jsonUnmarshalStd(b, v)
}

// stdlib import via small indirection to keep this file tidy
func jsonUnmarshalStd(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}
