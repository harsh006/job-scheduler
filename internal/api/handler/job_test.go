package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/harshRZP/job-scheduler/internal/domain"
	"github.com/harshRZP/job-scheduler/internal/repository"
	"github.com/harshRZP/job-scheduler/internal/scheduler"
)

// --- In-memory fakes ---

type fakeJobRepo struct {
	jobs map[string]domain.Job
}

func newFakeJobRepo() *fakeJobRepo {
	return &fakeJobRepo{jobs: make(map[string]domain.Job)}
}

func (r *fakeJobRepo) Create(_ context.Context, j domain.Job) error {
	r.jobs[j.ID] = j
	return nil
}
func (r *fakeJobRepo) GetByID(_ context.Context, id string) (domain.Job, error) {
	j, ok := r.jobs[id]
	if !ok {
		return domain.Job{}, repository.ErrNotFound
	}
	return j, nil
}
func (r *fakeJobRepo) Update(_ context.Context, j domain.Job) error {
	r.jobs[j.ID] = j
	return nil
}
func (r *fakeJobRepo) Delete(_ context.Context, id string) error {
	if j, ok := r.jobs[id]; ok {
		j.Status = domain.JobStatusDeleted
		r.jobs[id] = j
	}
	return nil
}
func (r *fakeJobRepo) ListActive(_ context.Context) ([]domain.Job, error) {
	var out []domain.Job
	for _, j := range r.jobs {
		if j.Status == domain.JobStatusActive {
			out = append(out, j)
		}
	}
	return out, nil
}
func (r *fakeJobRepo) List(_ context.Context) ([]domain.Job, error) {
	var out []domain.Job
	for _, j := range r.jobs {
		if j.Status != domain.JobStatusDeleted {
			out = append(out, j)
		}
	}
	return out, nil
}

type fakeNotifier struct{}

func (n *fakeNotifier) NotifyCreated(_ domain.Job) error  { return nil }
func (n *fakeNotifier) NotifyUpdated(_ domain.Job) error  { return nil }
func (n *fakeNotifier) NotifyDeleted(_ string) error      { return nil }

var _ repository.JobRepository = (*fakeJobRepo)(nil)
var _ scheduler.JobChangeNotifier = (*fakeNotifier)(nil)

// --- Helpers ---

func makeRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- Tests ---

func TestCreateJob(t *testing.T) {
	repo := newFakeJobRepo()
	h := NewJobHandler(repo, &fakeNotifier{})

	w := httptest.NewRecorder()
	req := makeRequest("POST", "/api/v1/jobs", map[string]any{
		"name":      "test-job",
		"cron_expr": "*/5 * * * *",
		"url":       "http://example.com",
	})

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var got domain.Job
	decodeBody(t, w, &got)

	if got.Name != "test-job" {
		t.Errorf("name mismatch: %s", got.Name)
	}
	if got.Status != domain.JobStatusActive {
		t.Errorf("expected active status, got %s", got.Status)
	}
	if got.MaxRetries != 3 {
		t.Errorf("expected default max_retries=3, got %d", got.MaxRetries)
	}
}

func TestCreateJobMissingName(t *testing.T) {
	repo := newFakeJobRepo()
	h := NewJobHandler(repo, &fakeNotifier{})

	w := httptest.NewRecorder()
	req := makeRequest("POST", "/api/v1/jobs", map[string]any{
		"cron_expr": "*/5 * * * *",
		"url":       "http://example.com",
	})

	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateJobInvalidCron(t *testing.T) {
	repo := newFakeJobRepo()
	h := NewJobHandler(repo, &fakeNotifier{})

	w := httptest.NewRecorder()
	req := makeRequest("POST", "/api/v1/jobs", map[string]any{
		"name":      "test",
		"cron_expr": "not-a-cron",
		"url":       "http://example.com",
	})

	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid cron, got %d", w.Code)
	}
}

func TestGetJob(t *testing.T) {
	repo := newFakeJobRepo()
	id := uuid.New().String()
	repo.jobs[id] = domain.Job{ID: id, Name: "my-job", Status: domain.JobStatusActive}

	h := NewJobHandler(repo, &fakeNotifier{})
	w := httptest.NewRecorder()
	req := withURLParam(makeRequest("GET", "/api/v1/jobs/"+id, nil), "id", id)

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetJobNotFound(t *testing.T) {
	h := NewJobHandler(newFakeJobRepo(), &fakeNotifier{})
	w := httptest.NewRecorder()
	req := withURLParam(makeRequest("GET", "/api/v1/jobs/missing", nil), "id", "missing")

	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteJob(t *testing.T) {
	repo := newFakeJobRepo()
	id := uuid.New().String()
	repo.jobs[id] = domain.Job{ID: id, Name: "to-delete", Status: domain.JobStatusActive}

	h := NewJobHandler(repo, &fakeNotifier{})
	w := httptest.NewRecorder()
	req := withURLParam(makeRequest("DELETE", "/api/v1/jobs/"+id, nil), "id", id)

	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if repo.jobs[id].Status != domain.JobStatusDeleted {
		t.Errorf("expected job to be soft-deleted")
	}
}

func TestPauseAndResumeJob(t *testing.T) {
	repo := newFakeJobRepo()
	id := uuid.New().String()
	repo.jobs[id] = domain.Job{
		ID:        id,
		Name:      "pausable",
		Status:    domain.JobStatusActive,
		CronExpr:  "*/1 * * * *",
		NextRunAt: time.Now().Add(time.Minute),
	}

	h := NewJobHandler(repo, &fakeNotifier{})

	// Pause
	w := httptest.NewRecorder()
	req := withURLParam(makeRequest("PATCH", "/api/v1/jobs/"+id+"/pause", nil), "id", id)
	h.Pause(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.jobs[id].Status != domain.JobStatusPaused {
		t.Errorf("expected paused status")
	}

	// Resume
	w = httptest.NewRecorder()
	req = withURLParam(makeRequest("PATCH", "/api/v1/jobs/"+id+"/resume", nil), "id", id)
	h.Resume(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.jobs[id].Status != domain.JobStatusActive {
		t.Errorf("expected active status after resume")
	}
}
