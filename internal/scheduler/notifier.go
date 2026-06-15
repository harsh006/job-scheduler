package scheduler

import "github.com/harshRZP/job-scheduler/internal/domain"

// JobChangeNotifier decouples the API layer from the scheduler internals.
// Swap InProcessNotifier for a RedisNotifier without changing any handler code.
type JobChangeNotifier interface {
	NotifyCreated(job domain.Job) error
	NotifyUpdated(job domain.Job) error
	NotifyDeleted(jobID string) error
}

// InProcessNotifier calls the Scheduler directly in the same process.
type InProcessNotifier struct {
	scheduler Scheduler
}

func NewInProcessNotifier(s Scheduler) *InProcessNotifier {
	return &InProcessNotifier{scheduler: s}
}

func (n *InProcessNotifier) NotifyCreated(job domain.Job) error {
	return n.scheduler.AddJob(job)
}

func (n *InProcessNotifier) NotifyUpdated(job domain.Job) error {
	return n.scheduler.UpdateJob(job)
}

func (n *InProcessNotifier) NotifyDeleted(jobID string) error {
	return n.scheduler.RemoveJob(jobID)
}
