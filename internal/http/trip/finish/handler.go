package finish

import (
	"log/slog"
	"net/http"

	api "github.com/Derunnikita1/template/internal/generated"
	"github.com/Derunnikita1/template/internal/http/response"
	"github.com/Derunnikita1/template/internal/http/trip/convert"
)

type Handler struct {
	useCase useCase
	log     *slog.Logger
}

func New(useCase useCase, log *slog.Logger) *Handler {
	return &Handler{useCase: useCase, log: log}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	entity, err := h.useCase.Execute(r.Context(), id)
	if err != nil {
		response.WriteError(h.log, w, r, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, convert.ToAPI(entity))
}
