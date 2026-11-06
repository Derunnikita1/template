package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Derunnikita1/template/internal/http/domainerror"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func WriteProblem(w http.ResponseWriter, status int, problem any) {
	body, err := json.Marshal(problem)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func WriteError(log *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err, r.URL.Path)
	if status >= http.StatusInternalServerError {
		log.ErrorContext(r.Context(), "request failed", "err", err)
	}
	WriteProblem(w, status, problem)
}
