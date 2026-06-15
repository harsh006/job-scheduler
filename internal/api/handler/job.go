package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/harshRZP/job-scheduler/internal/domain"
	"github.com/harshRZP/job-scheduler/internal/repository"
	"github.com/harshRZP/job-scheduler/internal/scheduler"
)

type JobHandler struct {
	jobRepo  repository.JobRepository
	notifier scheduler.JobChangeNotifier
}

func NewJobHandler(jobRepo repository.JobRepository, notifier scheduler.JobChangeNotifier) *JobHandler {
	return &JobHandler{jobRepo: jobRepo, notifier: notifier}
}

// createJobRequest is the request body for job creation.
type createJobRequest struct {
	Name           string            `json:"name"`
	CronExpr       string            `json:"cron_expr"`
	URL            string            `json:"url"`
	HTTPMethod     string            `json:"http_method"`
	Payload        string            `json:"payload"`
	Headers        map[string]string `json:"headers"`
	MaxRetries     *int              `json:"max_retries"`
	RetryDelaySecs *int              `json:"retry_delay_secs"`
}

func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateCreateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	nextRunAt, err := computeNextRunAt(req.CronExpr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now().UTC()
	job := domain.Job{
		ID:             uuid.New().String(),
		Name:           req.Name,
		CronExpr:       req.CronExpr,
		URL:            req.URL,
		HTTPMethod:     defaultMethod(req.HTTPMethod),
		Payload:        req.Payload,
		Headers:        req.Headers,
		Status:         domain.JobStatusActive,
		MaxRetries:     defaultInt(req.MaxRetries, 3),
		RetryDelaySecs: defaultInt(req.RetryDelaySecs, 5),
		NextRunAt:      nextRunAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.jobRepo.Create(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	if err := h.notifier.NotifyCreated(job); err != nil {
		// job is persisted; scheduler notification is best-effort
		// it will be picked up on next restart
		_ = err
	}

	writeJSON(w, http.StatusCreated, job)
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := h.jobRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.jobRepo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	if jobs == nil {
		jobs = []domain.Job{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (h *JobHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.jobRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CronExpr != "" && req.CronExpr != existing.CronExpr {
		nextRunAt, err := computeNextRunAt(req.CronExpr)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		existing.CronExpr = req.CronExpr
		existing.NextRunAt = nextRunAt
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.URL != "" {
		existing.URL = req.URL
	}
	if req.HTTPMethod != "" {
		existing.HTTPMethod = req.HTTPMethod
	}
	if req.Payload != "" {
		existing.Payload = req.Payload
	}
	if req.Headers != nil {
		existing.Headers = req.Headers
	}
	if req.MaxRetries != nil {
		existing.MaxRetries = *req.MaxRetries
	}
	if req.RetryDelaySecs != nil {
		existing.RetryDelaySecs = *req.RetryDelaySecs
	}

	if err := h.jobRepo.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job")
		return
	}

	_ = h.notifier.NotifyUpdated(existing)

	writeJSON(w, http.StatusOK, existing)
}

func (h *JobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if _, err := h.jobRepo.GetByID(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if err := h.jobRepo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	_ = h.notifier.NotifyDeleted(id)

	w.WriteHeader(http.StatusNoContent)
}

func (h *JobHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, domain.JobStatusPaused)
}

func (h *JobHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setStatus(w, r, domain.JobStatusActive)
}

func (h *JobHandler) setStatus(w http.ResponseWriter, r *http.Request, status domain.JobStatus) {
	id := chi.URLParam(r, "id")

	job, err := h.jobRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status == domain.JobStatusDeleted {
		writeError(w, http.StatusBadRequest, "cannot modify a deleted job")
		return
	}

	job.Status = status
	if err := h.jobRepo.Update(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job status")
		return
	}

	_ = h.notifier.NotifyUpdated(job)

	writeJSON(w, http.StatusOK, job)
}

func validateCreateRequest(req createJobRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.CronExpr == "" {
		return errors.New("cron_expr is required")
	}
	if req.URL == "" {
		return errors.New("url is required")
	}
	return nil
}

func computeNextRunAt(cronExpr string) (time.Time, error) {
	sched, err := scheduler.ParseSchedule(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(time.Now()), nil
}

func defaultMethod(m string) string {
	if m == "" {
		return "POST"
	}
	return m
}

func defaultInt(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}
