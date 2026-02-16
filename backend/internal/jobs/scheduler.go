// Package jobs provides background job scheduling.
package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// JobFunc is the function signature for jobs.
type JobFunc func(ctx context.Context) error

// Job represents a scheduled job.
type Job struct {
	Name     string
	Schedule string
	Func     JobFunc
	EntryID  cron.EntryID
}

// Scheduler manages background jobs.
type Scheduler struct {
	cron   *cron.Cron
	jobs   map[string]*Job
	logger *slog.Logger
	mu     sync.RWMutex
}

// NewScheduler creates a new job scheduler.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		jobs: make(map[string]*Job),
		logger: logger,
	}
}

// Register adds a job to the scheduler.
func (s *Scheduler) Register(name, schedule string, fn JobFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := &Job{
		Name:     name,
		Schedule: schedule,
		Func:     fn,
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.runJob(job)
	})
	if err != nil {
		return err
	}

	job.EntryID = entryID
	s.jobs[name] = job

	s.logger.Info("job registered", "name", name, "schedule", schedule)
	return nil
}

// Start starts the scheduler.
func (s *Scheduler) Start() error {
	s.cron.Start()
	s.logger.Info("scheduler started", "jobs", len(s.jobs))
	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler stopped")
}

// RunNow runs a job immediately.
func (s *Scheduler) RunNow(name string) error {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	go s.runJob(job)
	return nil
}

func (s *Scheduler) runJob(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	start := time.Now()
	s.logger.Info("job started", "name", job.Name)

	err := job.Func(ctx)

	duration := time.Since(start)
	if err != nil {
		s.logger.Error("job failed", "name", job.Name, "duration", duration, "error", err)
	} else {
		s.logger.Info("job completed", "name", job.Name, "duration", duration)
	}
}

// ListJobs returns all registered jobs.
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}
