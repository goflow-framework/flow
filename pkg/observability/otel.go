package observability

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SetupStdoutTracer sets up a simple stdout tracer for local/dev use and returns a shutdown func.
// It writes spans to stdout in a human-readable format. Use for local testing or CI dumps.
func SetupStdoutTracer(serviceName string) (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	res, err := resource.New(context.Background(),
		resource.WithAttributes(attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	shutdown := func(ctx context.Context) error {
		log.Printf("shutting down tracer provider for %s", serviceName)
		return tp.Shutdown(ctx)
	}
	return shutdown, nil
}

// Note: a real OTLP exporter and more options can be wired similarly (OTLP gRPC/HTTP).
// This file intentionally provides a small, dependency-light tracer setup for demos and tests.
