package finish_test

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
	finishhandler "github.com/Derunnikita1/template/internal/http/trip/finish"
)

func TestHandlerCompleted(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	id := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), id).Return(domaintrip.Trip{}, domaintrip.ErrTripCompleted)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)

	finishhandler.New(useCase, slog.New(slog.NewTextHandler(io.Discard, nil))).Handle(recorder, request, id)

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"trip_completed"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerSuccess(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	id := uuid.New()
	finishedAt := time.Now()
	useCase.EXPECT().Execute(gomock.Any(), id).Return(domaintrip.Trip{
		ID: id, UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusCompleted,
		StartedAt: finishedAt.Add(-time.Hour), FinishedAt: &finishedAt,
	}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)

	finishhandler.New(useCase, slog.New(slog.NewTextHandler(io.Discard, nil))).Handle(recorder, request, id)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
