package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Derunnikita1/template/internal/domain"
	api "github.com/Derunnikita1/template/internal/generated"
	"github.com/Derunnikita1/template/internal/repository"
	"github.com/Derunnikita1/template/internal/service"
)

type Handler struct {
	trips        *service.TripService
	pool         *pgxpool.Pool
	queryTimeout time.Duration
	log          *slog.Logger
}

func NewHandler(trips *service.TripService, pool *pgxpool.Pool, queryTimeout time.Duration, log *slog.Logger) *Handler {
	return &Handler{
		trips:        trips,
		pool:         pool,
		queryTimeout: queryTimeout,
		log:          log,
	}
}

func NewRouter(h *Handler) chi.Router {
	r := chi.NewRouter()
	api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
			instance := req.URL.Path
			detail := "Request validation failed"
			problem := api.Problem{
				Type:     "https://tripgo.example/problems/invalid-request",
				Title:    "Invalid request",
				Status:   http.StatusBadRequest,
				Detail:   &detail,
				Instance: &instance,
				Code:     "invalid_request",
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(problem)
		},
	})
	return r
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	var body api.TripData
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		h.writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request validation failed")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		h.writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request validation failed")
		return
	}

	if err := validateTripData(body); err != nil {
		h.writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request validation failed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	trip, err := h.trips.Create(ctx, service.CreateTripInput{
		UserID:         uuid.UUID(body.UserId),
		DriverID:       uuid.UUID(body.DriverId),
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	})
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	h.writeJSON(w, http.StatusCreated, toAPITrip(trip))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	trip, err := h.trips.Get(ctx, uuid.UUID(tripId))
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	h.writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	trip, err := h.trips.Finish(ctx, uuid.UUID(tripId))
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	h.writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		h.writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	h.writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		h.writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request validation failed")
	case errors.Is(err, domain.ErrTripNotFound):
		h.writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
	case errors.Is(err, domain.ErrTripCompleted):
		h.writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
	case errors.Is(err, domain.ErrDriverBusy):
		h.writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
	default:
		h.log.Error("request failed", "err", err, "path", r.URL.Path)
		h.writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
	}
}

func (h *Handler) writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	instance := r.URL.Path
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/" + problemType(code),
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}

func problemType(code string) string {
	switch code {
	case "invalid_request":
		return "invalid-request"
	case "trip_not_found":
		return "trip-not-found"
	case "trip_completed":
		return "trip-completed"
	case "driver_busy":
		return "driver-busy"
	case "idempotency_conflict":
		return "idempotency-conflict"
	default:
		return "internal-error"
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func validateTripData(body api.TripData) error {
	if body.UserId == (openapi_types.UUID{}) {
		return domain.ErrInvalidRequest
	}
	if body.DriverId == (openapi_types.UUID{}) {
		return domain.ErrInvalidRequest
	}
	if body.StartPoint.Latitude < -90 || body.StartPoint.Latitude > 90 {
		return domain.ErrInvalidRequest
	}
	if body.StartPoint.Longitude < -180 || body.StartPoint.Longitude > 180 {
		return domain.ErrInvalidRequest
	}
	if body.EndPoint.Latitude < -90 || body.EndPoint.Latitude > 90 {
		return domain.ErrInvalidRequest
	}
	if body.EndPoint.Longitude < -180 || body.EndPoint.Longitude > 180 {
		return domain.ErrInvalidRequest
	}
	if body.Price < 0 {
		return domain.ErrInvalidRequest
	}
	return nil
}

func toAPITrip(trip repository.Trip) api.Trip {
	return api.Trip{
		Id:       openapi_types.UUID(trip.ID),
		UserId:   openapi_types.UUID(trip.UserID),
		DriverId: openapi_types.UUID(trip.DriverID),
		StartPoint: api.Coordinates{
			Latitude:  trip.StartLatitude,
			Longitude: trip.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  trip.EndLatitude,
			Longitude: trip.EndLongitude,
		},
		Price:          trip.Price,
		Status:         api.TripStatus(trip.Status),
		StartedAt:      trip.StartedAt,
		FinishedAt:     trip.FinishedAt,
		LastPositionAt: trip.LastPositionAt,
	}
}
