package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"

	"github.com/Derunnikita1/template/internal/http/domainerror"
	"github.com/Derunnikita1/template/internal/http/response"
)

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						"panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
					response.WriteProblem(w, http.StatusInternalServerError, domainerror.Internal(r.URL.Path))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		instrumented := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				recovered := recover()
				status := responseStatus(wrapped.Status())
				if recovered != nil {
					status = http.StatusInternalServerError
				}
				route := routeTemplate(r)
				span := trace.SpanFromContext(r.Context())
				span.SetName(r.Method + " " + route)
				span.SetAttributes(attribute.String("http.route", route))
				if status >= http.StatusInternalServerError {
					span.RecordError(errors.New(http.StatusText(status)))
					span.SetStatus(codes.Error, http.StatusText(status))
					span.SetAttributes(attribute.String("error.type", http.StatusText(status)))
				}
				if recovered != nil {
					panic(recovered)
				}
			}()
			next.ServeHTTP(wrapped, r)
		}), "HTTP", otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()))
		return instrumented
	}
}

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				recovered := recover()
				status := responseStatus(wrapped.Status())
				if recovered != nil {
					status = http.StatusInternalServerError
				}
				args := []any{
					"method", r.Method,
					"route", routeTemplate(r),
					"status", status,
					"duration_ms", time.Since(startedAt).Milliseconds(),
				}
				if status >= http.StatusInternalServerError {
					log.ErrorContext(r.Context(), "request handled", args...)
				} else {
					log.InfoContext(r.Context(), "request handled", args...)
				}
				if recovered != nil {
					panic(recovered)
				}
			}()
			next.ServeHTTP(wrapped, r)
		})
	}
}

func Metrics(metrics metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			route := routeTemplate(r)
			metrics.HTTPStarted(r.Context(), r.Method, route)
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				recovered := recover()
				status := responseStatus(wrapped.Status())
				if recovered != nil {
					status = http.StatusInternalServerError
				}
				metrics.HTTPFinished(
					r.Context(), r.Method, routeTemplate(r), status, time.Since(startedAt),
				)
				if recovered != nil {
					panic(recovered)
				}
			}()
			next.ServeHTTP(wrapped, r)
		})
	}
}

func responseStatus(status int) int {
	if status == 0 {
		return http.StatusOK
	}
	return status
}

func routeTemplate(r *http.Request) string {
	if route := chi.RouteContext(r.Context()).RoutePattern(); route != "" {
		return route
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "trips" {
		return "/api/v1/trips/{tripId}"
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "trips" && parts[4] == "finish" {
		return "/api/v1/trips/{tripId}/finish"
	}
	return r.URL.Path
}
