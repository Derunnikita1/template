package telemetry

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	httpDuration metric.Float64Histogram
	httpActive   metric.Int64UpDownCounter
	tripsCreated metric.Int64Counter
	tripsDone    metric.Int64Counter
	activeTrips  atomic.Int64
}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	httpDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP duration metric: %w", err)
	}
	httpActive, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of active HTTP server requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active HTTP requests metric: %w", err)
	}
	tripsCreated, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create trips created metric: %w", err)
	}
	tripsDone, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create trips completed metric: %w", err)
	}

	metrics := &Metrics{
		httpDuration: httpDuration,
		httpActive:   httpActive,
		tripsCreated: tripsCreated,
		tripsDone:    tripsDone,
	}
	activeGauge, err := meter.Int64ObservableGauge(
		"tripgo.trips.active",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of active trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active trips metric: %w", err)
	}
	if _, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(activeGauge, metrics.activeTrips.Load())
		return nil
	}, activeGauge); err != nil {
		return nil, fmt.Errorf("register active trips metric: %w", err)
	}

	return metrics, nil
}

func (m *Metrics) HTTPStarted(ctx context.Context, method, route string) {
	m.httpActive.Add(ctx, 1, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	))
}

func (m *Metrics) HTTPFinished(ctx context.Context, method, route string, status int, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	)
	m.httpActive.Add(ctx, -1, attrs)
	m.httpDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.String("http.response.status_code", strconv.Itoa(status)),
	))
}

func (m *Metrics) TripCreated(ctx context.Context) {
	m.tripsCreated.Add(ctx, 1)
	m.activeTrips.Add(1)
}

func (m *Metrics) TripCompleted(ctx context.Context) {
	m.tripsDone.Add(ctx, 1)
	m.activeTrips.Add(-1)
}

func (m *Metrics) SetActiveTrips(count int64) {
	m.activeTrips.Store(count)
}
