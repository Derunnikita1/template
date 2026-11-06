package get_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	gethandler "github.com/Derunnikita1/template/internal/http/trip/get"
)

func TestHandlerNotFound(t *testing.T) {
	controller := gomock.NewController(t)
	useCase := NewMockuseCase(controller)
	id := uuid.New()
	useCase.EXPECT().Execute(gomock.Any(), id).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)

	gethandler.New(useCase, slog.New(slog.NewTextHandler(io.Discard, nil))).Handle(recorder, request, id)

	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"trip_not_found"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}
