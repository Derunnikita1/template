package list_test

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
	positionlisthandler "github.com/Derunnikita1/template/internal/http/position/list"
)

func TestHandlerReturnsRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	tripID := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), tripID).Return([]domaintrip.Position{
		{ID: 1, TripID: tripID, Latitude: 59.93, Longitude: 30.33, RecordedAt: time.Now()},
	}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)

	positionlisthandler.New(useCase, testLogger()).Handle(recorder, request, tripID)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"positions":[{"coordinates"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerReturnsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := NewMockuseCase(ctrl)
	tripID := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), tripID).Return(nil, domaintrip.ErrTripNotFound)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)

	positionlisthandler.New(useCase, testLogger()).Handle(recorder, request, tripID)

	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"trip_not_found"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
