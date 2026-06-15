package domain

import "time"

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusMissed    RunStatus = "missed"
)

type Run struct {
	ID           string
	JobID        string
	Status       RunStatus
	Attempt      int
	StartedAt    time.Time
	FinishedAt   *time.Time
	DurationMs   *int64
	ResponseCode *int
	ErrorMessage *string
}

type RunResult struct {
	Status       RunStatus
	ResponseCode int
	ErrorMessage string
	DurationMs   int64
	Attempts     int
}
