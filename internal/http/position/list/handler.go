package list

import (
	"log/slog"
	"net/http"

	api "github.com/Derunnikita1/template/internal/generated"
	"github.com/Derunnikita1/template/internal/http/position/convert"
	"github.com/Derunnikita1/template/internal/http/response"
)

type Handler struct {
	useCase useCase
	log     *slog.Logger
}

func New(useCase useCase, log *slog.Logger) *Handler {
	return &Handler{useCase: useCase, log: log}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	positions, err := h.useCase.Execute(r.Context(), tripID)
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, api.TripPositionsResponse{Positions: convert.ListToAPI(positions)})
}
