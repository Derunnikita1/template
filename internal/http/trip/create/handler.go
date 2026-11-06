package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
	"github.com/Derunnikita1/template/internal/http/response"
	"github.com/Derunnikita1/template/internal/http/trip/convert"
	createuc "github.com/Derunnikita1/template/internal/usecase/trip/create"
)

const (
	maxRequestBodySize = 1 << 20
	bodyField          = "body"
)

type Handler struct {
	useCase useCase
	log     *slog.Logger
}

func New(useCase useCase, log *slog.Logger) *Handler {
	return &Handler{useCase: useCase, log: log}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	body, err := decode(r)
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}

	entity, err := h.useCase.Execute(r.Context(), createuc.Input{
		UserID:         body.UserId,
		DriverID:       body.DriverId,
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	})
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+entity.ID.String())
	response.WriteJSON(w, http.StatusCreated, convert.ToAPI(entity))
}

func decode(r *http.Request) (api.TripData, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize+1))
	if err != nil || len(data) > maxRequestBodySize {
		return api.TripData{}, domaintrip.ValidationError{Field: bodyField, Reason: "cannot be read"}
	}

	var body api.TripData
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return api.TripData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains invalid JSON"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return api.TripData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains multiple JSON values"}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return api.TripData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains invalid JSON"}
	}
	if err := validateRequired(raw); err != nil {
		return api.TripData{}, err
	}

	return body, nil
}

func validateRequired(raw map[string]json.RawMessage) error {
	for _, field := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		value, ok := raw[field]
		if !ok || isNull(value) {
			return domaintrip.ValidationError{Field: field, Reason: "is required"}
		}
	}

	for _, field := range []string{"start_point", "end_point"} {
		var coordinates map[string]json.RawMessage
		if err := json.Unmarshal(raw[field], &coordinates); err != nil {
			return domaintrip.ValidationError{Field: field, Reason: "must be an object"}
		}
		for _, coordinate := range []string{"latitude", "longitude"} {
			value, ok := coordinates[coordinate]
			if !ok || isNull(value) {
				return domaintrip.ValidationError{Field: field + "." + coordinate, Reason: "is required"}
			}
		}
	}

	return nil
}

func isNull(value json.RawMessage) bool {
	return len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
