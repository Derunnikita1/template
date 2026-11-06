package create_test

import (
	"errors"
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
	api "github.com/Derunnikita1/template/internal/generated"
	createhandler "github.com/Derunnikita1/template/internal/http/trip/create"
)

func TestHandlerSuccess(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	id := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{
		ID: id, UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusActive,
		StartedAt: time.Now(),
	}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody()))
	createhandler.New(useCase, handlerLogger()).Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if recorder.Header().Get("Location") != "/api/v1/trips/"+id.String() {
		t.Fatalf("unexpected Location header: %q", recorder.Header().Get("Location"))
	}
}

func TestHandlerInvalidBody(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(`{"user_id":null}`))

	createhandler.New(useCase, handlerLogger()).Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerDriverBusy(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{}, domaintrip.ErrDriverBusy)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody()))

	createhandler.New(useCase, handlerLogger()).Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"driver_busy"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerInternalErrorDoesNotLeakDetails(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	secret := "postgres password is secret"
	useCase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{}, errors.New(secret))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody()))

	createhandler.New(useCase, handlerLogger()).Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatal("internal error details leaked into response")
	}
}

func validBody() string {
	return `{"user_id":"11111111-1111-1111-1111-111111111111","driver_id":"22222222-2222-2222-2222-222222222222","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.4},"price":1450}`
}

func handlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
