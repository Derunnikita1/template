package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
	appmiddleware "github.com/Derunnikita1/template/internal/http/middleware"
	"github.com/Derunnikita1/template/internal/http/response"
	createhandler "github.com/Derunnikita1/template/internal/http/trip/create"
	finishhandler "github.com/Derunnikita1/template/internal/http/trip/finish"
	gethandler "github.com/Derunnikita1/template/internal/http/trip/get"
)

type Config struct {
	QueryTimeout time.Duration
}

type Handler struct {
	create    *createhandler.Handler
	get       *gethandler.Handler
	finish    *finishhandler.Handler
	readiness readiness
	config    Config
}

func NewHandler(
	create *createhandler.Handler,
	get *gethandler.Handler,
	finish *finishhandler.Handler,
	readiness readiness,
	config Config,
) *Handler {
	return &Handler{create: create, get: get, finish: finish, readiness: readiness, config: config}
}

func NewRouter(handler *Handler, log *slog.Logger, metrics appmiddlewareMetrics) chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.Recover(log),
		appmiddleware.Tracing(),
		appmiddleware.Logging(log),
		appmiddleware.Metrics(metrics),
	)
	api.HandlerWithOptions(handler, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			response.WriteError(log, w, r, domaintrip.ValidationError{
				Field: "request", Reason: "cannot be parsed",
			})
		},
	})
	return router
}

type appmiddlewareMetrics interface {
	HTTPStarted(ctx context.Context, method, route string)
	HTTPFinished(ctx context.Context, method, route string, status int, duration time.Duration)
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	h.create.Handle(w, r, params)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.get.Handle(w, r, id)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	h.finish.Handle(w, r, id)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.config.QueryTimeout)
	defer cancel()
	if err := h.readiness.Ping(ctx); err != nil {
		response.WriteJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	response.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
