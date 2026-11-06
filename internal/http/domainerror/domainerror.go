package domainerror

import (
	"errors"
	"net/http"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
)

func Problem(err error, instance string) (int, api.Problem) {
	status := http.StatusInternalServerError
	code := "internal_error"
	title := "Internal Server Error"
	detail := "Internal server error"
	typeName := "internal-error"

	var validationError domaintrip.ValidationError
	switch {
	case errors.As(err, &validationError):
		status = http.StatusBadRequest
		code = "invalid_request"
		title = "Invalid request"
		detail = "Request validation failed"
		typeName = "invalid-request"
	case errors.Is(err, domaintrip.ErrTripNotFound):
		status = http.StatusNotFound
		code = "trip_not_found"
		title = "Trip not found"
		detail = "Trip was not found"
		typeName = "trip-not-found"
	case errors.Is(err, domaintrip.ErrTripCompleted):
		status = http.StatusConflict
		code = "trip_completed"
		title = "Trip completed"
		detail = "Operation is not allowed for a completed trip"
		typeName = "trip-completed"
	case errors.Is(err, domaintrip.ErrDriverBusy):
		status = http.StatusConflict
		code = "driver_busy"
		title = "Driver busy"
		detail = "Driver already has an active trip"
		typeName = "driver-busy"
	}

	return status, api.Problem{
		Type:     "https://tripgo.example/problems/" + typeName,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &instance,
		Code:     code,
	}
}

func Internal(instance string) api.Problem {
	_, problem := Problem(errors.New("internal error"), instance)
	return problem
}
