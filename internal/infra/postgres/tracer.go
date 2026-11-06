package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type queryContextKey struct{}

type queryContext struct {
	startedAt  time.Time
	operation  string
	collection string
}

type queryTracer struct {
	base     pgx.QueryTracer
	duration metric.Float64Histogram
}

func newQueryTracer(meter metric.Meter) (*queryTracer, error) {
	duration, err := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of database client operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("create database duration metric: %w", err)
	}

	return &queryTracer{
		base: otelpgx.NewTracer(
			otelpgx.WithMeterProvider(metricnoop.NewMeterProvider()),
			otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
		),
		duration: duration,
	}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.base.TraceQueryStart(ctx, conn, data)
	operation := otelpgx.SQLOperationName(ctx, data.SQL)
	collection := collectionName(data.SQL)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("db.operation.name", operation),
		attribute.String("db.collection.name", collection),
	)
	return context.WithValue(ctx, queryContextKey{}, queryContext{
		startedAt: time.Now(), operation: operation, collection: collection,
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	query, ok := ctx.Value(queryContextKey{}).(queryContext)
	if ok {
		errorType := ""
		if data.Err != nil {
			errorType = fmt.Sprintf("%T", data.Err)
			var databaseError *pgconn.PgError
			if ok := errorAs(data.Err, &databaseError); ok {
				errorType = databaseError.Code
			}
			trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", errorType))
		}
		t.duration.Record(ctx, time.Since(query.startedAt).Seconds(), metric.WithAttributes(
			attribute.String("db.operation.name", query.operation),
			attribute.String("db.collection.name", query.collection),
			attribute.String("error.type", errorType),
		))
	}
	t.base.TraceQueryEnd(ctx, conn, data)
}

func collectionName(query string) string {
	fields := strings.Fields(strings.NewReplacer("\n", " ", "\t", " ", ",", " ").Replace(query))
	for index, field := range fields {
		switch strings.ToUpper(field) {
		case "FROM", "INTO", "UPDATE":
			if index+1 < len(fields) {
				return strings.Trim(fields[index+1], `"`)
			}
		}
	}
	return "unknown"
}
