package save

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
	"github.com/Derunnikita1/template/internal/http/position/convert"
	"github.com/Derunnikita1/template/internal/http/response"
	positionsave "github.com/Derunnikita1/template/internal/usecase/position/save"
)

const maxRequestBodySize = 1 << 20

const bodyField = "body"

type Handler struct {
	useCase useCase
	log     *slog.Logger
}

func New(useCase useCase, log *slog.Logger) *Handler {
	return &Handler{useCase: useCase, log: log}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	body, err := decode(r)
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}

	entity, err := h.useCase.Execute(r.Context(), positionsave.Input{
		TripID:     tripID,
		Latitude:   body.Coordinates.Latitude,
		Longitude:  body.Coordinates.Longitude,
		RecordedAt: body.RecordedAt,
	})
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, convert.ToAPI(entity))
}

func decode(r *http.Request) (api.PositionData, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize+1))
	if err != nil || len(data) > maxRequestBodySize {
		return api.PositionData{}, domaintrip.ValidationError{Field: bodyField, Reason: "cannot be read"}
	}

	var body api.PositionData
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return api.PositionData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains invalid JSON"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return api.PositionData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains multiple JSON values"}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return api.PositionData{}, domaintrip.ValidationError{Field: bodyField, Reason: "contains invalid JSON"}
	}
	for _, field := range []string{"coordinates", "recorded_at"} {
		value, ok := raw[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return api.PositionData{}, domaintrip.ValidationError{Field: field, Reason: "is required"}
		}
	}
	var coordinates map[string]json.RawMessage
	if err := json.Unmarshal(raw["coordinates"], &coordinates); err != nil {
		return api.PositionData{}, domaintrip.ValidationError{Field: "coordinates", Reason: "must be an object"}
	}
	for _, field := range []string{"latitude", "longitude"} {
		value, ok := coordinates[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return api.PositionData{}, domaintrip.ValidationError{Field: "coordinates." + field, Reason: "is required"}
		}
	}
	return body, nil
}
