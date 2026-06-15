package domain

import "time"

type JobStatus string

const (
	JobStatusActive  JobStatus = "active"
	JobStatusPaused  JobStatus = "paused"
	JobStatusDeleted JobStatus = "deleted"
)

type Job struct {
	ID             string
	Name           string
	CronExpr       string
	URL            string
	HTTPMethod     string
	Payload        string
	Headers        map[string]string
	Status         JobStatus
	MaxRetries     int
	RetryDelaySecs int
	NextRunAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
