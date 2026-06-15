package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

func newJob(url, method string) domain.Job {
	return domain.Job{
		ID:             uuid.New().String(),
		URL:            url,
		HTTPMethod:     method,
		MaxRetries:     3,
		RetryDelaySecs: 0, // no wait between retries in tests
	}
}

func TestHTTPExecutorSuccessOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := NewHTTPExecutor(5)
	result := exec.Execute(context.Background(), newJob(srv.URL, "POST"))

	if result.Status != domain.RunStatusSucceeded {
		t.Errorf("expected succeeded, got %s", result.Status)
	}
	if result.ResponseCode != 200 {
		t.Errorf("expected 200, got %d", result.ResponseCode)
	}
}

func TestHTTPExecutorRetriesOn500ThenFails(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	job := newJob(srv.URL, "POST")
	job.MaxRetries = 3

	exec := NewHTTPExecutor(5)
	result := exec.Execute(context.Background(), job)

	if result.Status != domain.RunStatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if int(attempts.Load()) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestHTTPExecutorSucceedsOnSecondAttempt(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := NewHTTPExecutor(5)
	result := exec.Execute(context.Background(), newJob(srv.URL, "POST"))

	if result.Status != domain.RunStatusSucceeded {
		t.Errorf("expected succeeded on retry, got %s", result.Status)
	}
	if int(attempts.Load()) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestHTTPExecutorRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	exec := NewHTTPExecutor(5)
	result := exec.Execute(ctx, newJob(srv.URL, "POST"))

	if result.Status != domain.RunStatusFailed {
		t.Errorf("expected failed on timeout, got %s", result.Status)
	}
}

func TestHTTPExecutorSendsPayloadAndHeaders(t *testing.T) {
	var gotContentType, gotCustomHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotCustomHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	job := newJob(srv.URL, "POST")
	job.Payload = `{"key":"value"}`
	job.Headers = map[string]string{"X-Custom": "test"}

	exec := NewHTTPExecutor(5)
	_ = exec.Execute(context.Background(), job)

	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
	if gotCustomHeader != "test" {
		t.Errorf("expected X-Custom: test, got %q", gotCustomHeader)
	}
}
