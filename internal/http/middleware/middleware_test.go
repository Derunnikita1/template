package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	appmiddleware "github.com/Derunnikita1/template/internal/http/middleware"
)

type recordedMetrics struct {
	startedRoute  string
	finishedRoute string
	status        int
}

func (m *recordedMetrics) HTTPStarted(_ context.Context, _, route string) {
	m.startedRoute = route
}

func (m *recordedMetrics) HTTPFinished(_ context.Context, _, route string, status int, _ time.Duration) {
	m.finishedRoute = route
	m.status = status
}

func TestRecoverRecordsRequestAsInternalError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	metrics := &recordedMetrics{}
	router := chi.NewRouter()
	router.Use(
		appmiddleware.Recover(logger),
		appmiddleware.Tracing(),
		appmiddleware.Logging(logger),
		appmiddleware.Metrics(metrics),
	)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("failure") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	if metrics.startedRoute != "/panic" || metrics.finishedRoute != "/panic" || metrics.status != http.StatusInternalServerError {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
	if !strings.Contains(logs.String(), `"msg":"request handled"`) ||
		!strings.Contains(logs.String(), `"status":500`) ||
		!strings.Contains(logs.String(), `"msg":"panic recovered"`) {
		t.Fatalf("unexpected logs: %s", logs.String())
	}
}
