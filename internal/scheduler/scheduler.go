package scheduler

import "github.com/harshRZP/job-scheduler/internal/domain"

type Scheduler interface {
	Start() error
	Stop()
	AddJob(job domain.Job) error
	RemoveJob(jobID string) error
	UpdateJob(job domain.Job) error
}
