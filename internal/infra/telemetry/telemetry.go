package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	apimetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

const scope = "github.com/Derunnikita1/template"

type Config struct {
	ServiceName string
	Version     string
	LogLevel    slog.Level
	Sampler     string
}

type Provider struct {
	Logger  *slog.Logger
	Metrics *Metrics
	Meter   apimetric.MeterProvider
	traces  *trace.TracerProvider
	metrics *sdkmetric.MeterProvider
}

func New(ctx context.Context, cfg Config) (*Provider, error) {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	logger := slog.New(traceHandler{base: baseHandler}).With("service", cfg.ServiceName)

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.Version),
			attribute.String("service.namespace", "tripgo"),
			attribute.String("deployment.environment", "local"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(2*time.Second))
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	traceProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(sampler(cfg.Sampler)),
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(time.Second)),
	)

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second))
	if err != nil {
		_ = traceProvider.Shutdown(ctx)
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}
	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
	)

	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(metricProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn("telemetry export failed", "err", err)
	}))

	appMetrics, err := NewMetrics(metricProvider.Meter(scope))
	if err != nil {
		_ = errors.Join(metricProvider.Shutdown(ctx), traceProvider.Shutdown(ctx))
		return nil, err
	}

	return &Provider{
		Logger:  logger,
		Metrics: appMetrics,
		Meter:   metricProvider,
		traces:  traceProvider,
		metrics: metricProvider,
	}, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	return errors.Join(p.metrics.Shutdown(ctx), p.traces.Shutdown(ctx))
}

func sampler(name string) trace.Sampler {
	switch name {
	case "always_off":
		return trace.NeverSample()
	case "always_on":
		return trace.AlwaysSample()
	case "parentbased_always_off":
		return trace.ParentBased(trace.NeverSample())
	default:
		return trace.ParentBased(trace.AlwaysSample())
	}
}
