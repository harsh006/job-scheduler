package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/harshRZP/job-scheduler/internal/repository"
)

type RunHandler struct {
	runRepo repository.RunRepository
}

func NewRunHandler(runRepo repository.RunRepository) *RunHandler {
	return &RunHandler{runRepo: runRepo}
}

func (h *RunHandler) ListByJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	limit := queryIntOrDefault(r, "limit", 20)

	runs, err := h.runRepo.ListByJobID(r.Context(), jobID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch runs")
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *RunHandler) ListRecent(w http.ResponseWriter, r *http.Request) {
	limit := queryIntOrDefault(r, "limit", 50)

	runs, err := h.runRepo.ListRecent(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch runs")
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func queryIntOrDefault(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
