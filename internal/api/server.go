package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/harshRZP/job-scheduler/internal/api/handler"
	"github.com/harshRZP/job-scheduler/internal/api/middleware"
)

func NewServer(
	jobH *handler.JobHandler,
	runH *handler.RunHandler,
	auth middleware.Authenticator,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(middleware.BearerMiddleware(auth))

		r.Post("/api/v1/jobs", jobH.Create)
		r.Get("/api/v1/jobs", jobH.List)
		r.Get("/api/v1/jobs/{id}", jobH.Get)
		r.Put("/api/v1/jobs/{id}", jobH.Update)
		r.Delete("/api/v1/jobs/{id}", jobH.Delete)
		r.Patch("/api/v1/jobs/{id}/pause", jobH.Pause)
		r.Patch("/api/v1/jobs/{id}/resume", jobH.Resume)

		r.Get("/api/v1/jobs/{id}/runs", runH.ListByJob)
		r.Get("/api/v1/runs", runH.ListRecent)
	})

	return r
}
