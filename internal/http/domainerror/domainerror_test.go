package domainerror_test

import (
	"errors"
	"net/http"
	"testing"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	"github.com/Derunnikita1/template/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "validation", err: domaintrip.ValidationError{Field: "price", Reason: "invalid"}, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "not found", err: domaintrip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: "trip_not_found"},
		{name: "completed", err: domaintrip.ErrTripCompleted, wantStatus: http.StatusConflict, wantCode: "trip_completed"},
		{name: "driver busy", err: domaintrip.ErrDriverBusy, wantStatus: http.StatusConflict, wantCode: "driver_busy"},
		{name: "internal", err: errors.New("database password leaked"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			status, problem := domainerror.Problem(test.err, "/test")
			if status != test.wantStatus || problem.Code != test.wantCode {
				t.Fatalf("got status %d code %q, want status %d code %q", status, problem.Code, test.wantStatus, test.wantCode)
			}
			if problem.Detail != nil && *problem.Detail == test.err.Error() {
				t.Fatal("internal error details leaked into response")
			}
		})
	}
}
