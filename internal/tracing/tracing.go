package tracing

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer initializes OpenTelemetry tracing with stdout exporter.
// This provides distributed tracing without requiring external infrastructure.
// Returns a TracerProvider that should be shut down when the application exits.
func InitTracer(serviceName, serviceVersion string, logger *slog.Logger) (*trace.TracerProvider, error) {
	ctx := context.Background()

	// Use stdout exporter - no infrastructure required
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	if logger != nil {
		logger.Info("Tracing initialized",
			"service", serviceName,
			"version", serviceVersion,
			"exporter", "stdout",
		)
	}

	return tp, nil
}

// Shutdown gracefully shuts down the tracer provider.
func Shutdown(ctx context.Context, tp *trace.TracerProvider) error {
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}
