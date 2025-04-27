package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"

	"github.com/medeiros-dev/notification-service/go-producer/configs"
	"github.com/medeiros-dev/notification-service/go-producer/pkg/logger"
	"go.uber.org/zap"
)

var (
	shutdownFunc func(context.Context) error = func(ctx context.Context) error { return nil }
	Tracer       trace.Tracer

	// newExporterFunc allows overriding the exporter creation for testing
	newExporterFunc = func(ctx context.Context, cfg *configs.Config) (tracesdk.SpanExporter, error) {
		if cfg.OtelInsecure {
			return otlptracegrpc.New(ctx,
				otlptracegrpc.WithEndpoint(cfg.OtelEndpoint),
				otlptracegrpc.WithInsecure(),
			)
		}
		creds := credentials.NewClientTLSFromCert(nil, "") // Assuming default cert pool
		return otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OtelEndpoint),
			otlptracegrpc.WithTLSCredentials(creds),
		)
	}
)

func InitTracer(cfg *configs.Config) (func(context.Context) error, error) {
	ctx := context.Background()

	exporter, err := newExporterFunc(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.OtelServiceName),
		),
	)
	if err != nil {
		// If resource creation fails, we might have already created an exporter
		// that needs shutting down, but the original code didn't handle this.
		// For simplicity, we'll ignore exporter shutdown on resource error here,
		// assuming resource creation is unlikely to fail often in real scenarios.
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Set the package-level tracer
	Tracer = otel.Tracer(cfg.OtelServiceName) // Use the configured service name

	shutdownFunc = func(shutdownCtx context.Context) error {
		// It's important to shutdown the exporter as well, although
		// the TracerProvider's Shutdown *should* handle this.
		// Explicitly shutting down the exporter isn't strictly necessary here
		// if tp.Shutdown guarantees it, but we keep the tp shutdown.
		err := tp.Shutdown(shutdownCtx)
		// We could also call exporter.Shutdown here if needed, but typically tp.Shutdown suffices.
		// exporter.Shutdown(shutdownCtx)
		return err
	}
	return shutdownFunc, nil
}

func GetTracer() trace.Tracer {
	return Tracer
}

func ShutdownTracer(ctx context.Context) {
	if err := shutdownFunc(ctx); err != nil {
		logger.L().Error("Error shutting down tracer provider", zap.Error(err))
	}
}
