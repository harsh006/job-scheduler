package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

type HTTPExecutor struct {
	client *http.Client
}

func NewHTTPExecutor(timeoutSec int) *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (e *HTTPExecutor) Execute(ctx context.Context, job domain.Job) domain.RunResult {
	start := time.Now()

	var lastErr error
	var lastCode int
	var attempts int

	for attempt := 1; attempt <= job.MaxRetries; attempt++ {
		attempts = attempt
		code, err := e.doRequest(ctx, job)
		lastCode = code
		lastErr = err

		if err == nil && isSuccess(code) {
			return domain.RunResult{
				Status:       domain.RunStatusSucceeded,
				ResponseCode: code,
				DurationMs:   time.Since(start).Milliseconds(),
				Attempts:     attempts,
			}
		}

		if attempt < job.MaxRetries {
			select {
			case <-time.After(time.Duration(job.RetryDelaySecs) * time.Second):
			case <-ctx.Done():
				return domain.RunResult{
					Status:       domain.RunStatusFailed,
					ResponseCode: lastCode,
					ErrorMessage: "context cancelled during retry wait",
					DurationMs:   time.Since(start).Milliseconds(),
					Attempts:     attempts,
				}
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	} else {
		errMsg = fmt.Sprintf("non-2xx response: %d", lastCode)
	}

	return domain.RunResult{
		Status:       domain.RunStatusFailed,
		ResponseCode: lastCode,
		ErrorMessage: errMsg,
		DurationMs:   time.Since(start).Milliseconds(),
		Attempts:     attempts,
	}
}

func (e *HTTPExecutor) doRequest(ctx context.Context, job domain.Job) (int, error) {
	var body io.Reader
	if job.Payload != "" {
		body = bytes.NewBufferString(job.Payload)
	}

	req, err := http.NewRequestWithContext(ctx, job.HTTPMethod, job.URL, body)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	if job.Payload != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range job.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func isSuccess(code int) bool {
	return code >= 200 && code < 300
}
