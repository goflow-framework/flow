package observability

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// SetupOTLPTracer configures an OTLP/gRPC tracer exporter that sends spans to
	// the provided endpoint. If insecure is true, the connection will use insecure
	// credentials (useful for local collectors). headers may be used to supply
	// authentication metadata like API keys. Returns a shutdown function that must
	// be called to flush and stop the tracer provider.
	func SetupOTLPTracer(ctx context.Context, endpoint string, insecureConn bool, headers map[string]string, serviceName string) (func(context.Context) error, error) {
		opts := []otlptracegrpc.Option{}
		if endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
		}
		if insecureConn {
			opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		}
		if len(headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(headers))
		}

		exporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, err
		}

		res, err := resource.New(ctx,
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
			log.Printf("shutting down OTLP tracer provider for %s", serviceName)
			return tp.Shutdown(ctx)
		}
		return shutdown, nil
	}
