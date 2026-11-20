package save_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	positionsavehandler "github.com/Derunnikita1/template/internal/http/position/save"
)

func TestHandlerCreatesPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	tripID := uuid.New()
	recordedAt := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Position{
		ID: 42, TripID: tripID, Latitude: 59.93, Longitude: 30.33, RecordedAt: recordedAt,
	}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+tripID.String()+"/positions",
		strings.NewReader(`{"coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-20T12:00:00Z"}`),
	)

	positionsavehandler.New(useCase, testLogger()).Handle(recorder, request, tripID)

	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"id":42`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsInvalidPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	tripID := uuid.New()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+tripID.String()+"/positions",
		strings.NewReader(`{"coordinates":{"latitude":59.93}}`),
	)

	positionsavehandler.New(useCase, testLogger()).Handle(recorder, request, tripID)

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsCompletedTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	tripID := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Position{}, domaintrip.ErrTripCompleted)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+tripID.String()+"/positions",
		strings.NewReader(`{"coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-20T12:00:00Z"}`),
	)

	positionsavehandler.New(useCase, testLogger()).Handle(recorder, request, tripID)

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"trip_completed"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
