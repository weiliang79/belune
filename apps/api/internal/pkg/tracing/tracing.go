// Package tracing wires OpenTelemetry tracing into the server. When
// OTEL_EXPORTER_OTLP_ENDPOINT is unset the package installs a no-op provider —
// tracing calls remain cheap and the program runs without an exporter.
//
// When the endpoint is set, spans are sent via OTLP/HTTP. This pairs with the
// Phase 4 observability stack (Prometheus + Grafana + optional Jaeger/Tempo).
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerName is the instrumentation scope used for every span produced by
// first-party server code.
const TracerName = "belune/api"

// Config controls exporter wiring. Zero value is safe — tracing remains no-op.
type Config struct {
	// Endpoint is an OTLP/HTTP endpoint (e.g. "otel-collector:4318"). Empty = no-op.
	Endpoint string
	// Insecure skips TLS when talking to the collector. Leave true for
	// loopback-only collectors; flip for external endpoints.
	Insecure bool
	// ServiceName is reported as the resource service.name attribute.
	ServiceName string
	// ServiceVersion is reported as the resource service.version attribute.
	ServiceVersion string
}

// ShutdownFunc flushes buffered spans and releases exporter resources.
type ShutdownFunc func(context.Context) error

// Init installs a global tracer provider and W3C propagator. Returns a
// shutdown function that must be called before process exit.
//
// When cfg.Endpoint is empty, a no-op provider is installed and the returned
// shutdown function is a cheap no-op. Callers do not need to branch on this.
func Init(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	// Always install the W3C propagator so callers can inject/extract even
	// when no exporter is configured — the context still flows through.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.Endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		slog.Info("tracing: no OTLP endpoint configured, using no-op provider")
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	// Pass an empty schema URL on our resource so Merge adopts resource.Default()'s
	// SchemaURL. Hard-coding a specific semconv version here causes a merge
	// conflict whenever the OTel SDK bumps its default schema.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	slog.Info("tracing: OTLP exporter configured", "endpoint", cfg.Endpoint, "insecure", cfg.Insecure)

	return func(shutdownCtx context.Context) error {
		return tp.Shutdown(shutdownCtx)
	}, nil
}

// Tracer returns the package's named tracer. Safe to call before Init — the
// default no-op provider satisfies the interface.
func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// InjectContext serializes the active trace context from ctx into a flat
// string map suitable for storing in an asynq task payload.
//
// Returns an empty map if no span is active.
func InjectContext(ctx context.Context) map[string]string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier
}

// ExtractContext deserializes a carrier produced by InjectContext back into a
// context that can be used to start child spans.
func ExtractContext(ctx context.Context, carrier map[string]string) context.Context {
	if len(carrier) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}
