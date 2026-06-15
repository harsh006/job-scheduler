package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/harshRZP/job-scheduler/internal/domain"
	"github.com/harshRZP/job-scheduler/internal/executor"
	"github.com/harshRZP/job-scheduler/internal/repository"
)

type MinHeapScheduler struct {
	mu       sync.Mutex
	heap     entryHeap
	byID     map[string]*entry // fast lookup for remove/update
	notify   chan struct{}      // signals the loop that the heap changed
	stopCh   chan struct{}
	executor executor.JobExecutor
	runRepo  repository.RunRepository
}

func NewMinHeapScheduler(exec executor.JobExecutor, runRepo repository.RunRepository) *MinHeapScheduler {
	return &MinHeapScheduler{
		heap:     newEntryHeap(),
		byID:     make(map[string]*entry),
		notify:   make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
		executor: exec,
		runRepo:  runRepo,
	}
}

func (s *MinHeapScheduler) Start() error {
	go s.run()
	return nil
}

func (s *MinHeapScheduler) Stop() {
	close(s.stopCh)
}

func (s *MinHeapScheduler) AddJob(job domain.Job) error {
	sched, err := ParseSchedule(job.CronExpr)
	if err != nil {
		return err
	}

	e := &entry{
		job:      job,
		schedule: sched,
		next:     sched.Next(time.Now()),
	}

	s.mu.Lock()
	heap.Push(&s.heap, e)
	s.byID[job.ID] = e
	s.mu.Unlock()

	s.wake()
	return nil
}

func (s *MinHeapScheduler) RemoveJob(jobID string) error {
	s.mu.Lock()
	e, ok := s.byID[jobID]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	heap.Remove(&s.heap, e.index)
	delete(s.byID, jobID)
	s.mu.Unlock()

	s.wake()
	return nil
}

func (s *MinHeapScheduler) UpdateJob(job domain.Job) error {
	if err := s.RemoveJob(job.ID); err != nil {
		return err
	}
	if job.Status == domain.JobStatusActive {
		return s.AddJob(job)
	}
	return nil
}

// run is the single goroutine that owns the scheduling loop.
// It sleeps until the next job is due (or is woken early by notify/stop).
func (s *MinHeapScheduler) run() {
	for {
		s.mu.Lock()
		now := time.Now()

		// fire all entries whose next <= now
		for s.heap.Len() > 0 && !s.heap[0].next.After(now) {
			e := heap.Pop(&s.heap).(*entry)
			delete(s.byID, e.job.ID)

			go s.fire(e.job)

			// re-schedule: compute next and push back
			e.next = e.schedule.Next(now)
			heap.Push(&s.heap, e)
			s.byID[e.job.ID] = e
		}

		// decide how long to sleep
		var timer *time.Timer
		if s.heap.Len() == 0 {
			timer = time.NewTimer(24 * time.Hour) // nothing scheduled; wake on notify
		} else {
			timer = time.NewTimer(time.Until(s.heap[0].next))
		}
		s.mu.Unlock()

		select {
		case <-timer.C:
		case <-s.notify:
			timer.Stop()
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}
}

// fire executes one job run concurrently and records it in the run history.
func (s *MinHeapScheduler) fire(job domain.Job) {
	ctx := context.Background()
	runID := uuid.New().String()
	startedAt := time.Now().UTC()

	run := domain.Run{
		ID:        runID,
		JobID:     job.ID,
		Status:    domain.RunStatusRunning,
		Attempt:   1,
		StartedAt: startedAt,
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		log.Printf("scheduler: create run record for job %s: %v", job.ID, err)
	}

	result := s.executor.Execute(ctx, job)

	finishedAt := time.Now().UTC()
	durationMs := result.DurationMs
	responseCode := result.ResponseCode

	run.Status = result.Status
	run.FinishedAt = &finishedAt
	run.DurationMs = &durationMs
	run.ResponseCode = &responseCode
	if result.ErrorMessage != "" {
		run.ErrorMessage = &result.ErrorMessage
	}

	if err := s.runRepo.UpdateStatus(ctx, run); err != nil {
		log.Printf("scheduler: update run record %s: %v", runID, err)
	}
}

// wake sends a non-blocking signal to the scheduling loop to re-evaluate.
func (s *MinHeapScheduler) wake() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// ParseSchedule parses a standard 5-field cron expression.
// Exported so callers (e.g. API handlers) can validate expressions
// and compute next-run times without depending on the full engine.
func ParseSchedule(expr string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return sched, nil
}
