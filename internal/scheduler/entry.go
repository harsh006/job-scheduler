package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/harshRZP/job-scheduler/internal/domain"
)

// entry is a single slot in the min-heap.
// index tracks the entry's current position in the heap slice
// so heap.Fix can be called in O(log n) after an in-place update.
type entry struct {
	job      domain.Job
	schedule cron.Schedule // robfig Schedule; Next(t) computes the next trigger
	next     time.Time
	index    int // maintained by entryHeap.Swap
}
