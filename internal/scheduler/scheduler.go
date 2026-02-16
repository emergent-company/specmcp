// Package scheduler provides a simple task scheduler for periodic jobs.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job represents a scheduled task.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// Scheduler runs jobs on a periodic basis.
type Scheduler struct {
	logger *slog.Logger
	jobs   []scheduledJob
}

type scheduledJob struct {
	job      Job
	interval time.Duration
	ticker   *time.Ticker
	stop     chan struct{}
}

// NewScheduler creates a new scheduler.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		jobs:   make([]scheduledJob, 0),
	}
}

// AddJob adds a job to run at the specified interval.
func (s *Scheduler) AddJob(job Job, interval time.Duration) {
	s.jobs = append(s.jobs, scheduledJob{
		job:      job,
		interval: interval,
		stop:     make(chan struct{}),
	})
}

// Start begins running all scheduled jobs.
func (s *Scheduler) Start(ctx context.Context) {
	for i := range s.jobs {
		sj := &s.jobs[i]
		sj.ticker = time.NewTicker(sj.interval)

		go func(sj *scheduledJob) {
			s.logger.Info("starting scheduled job",
				"job", sj.job.Name(),
				"interval", sj.interval)

			for {
				select {
				case <-sj.ticker.C:
					s.logger.Debug("running scheduled job", "job", sj.job.Name())
					if err := sj.job.Run(ctx); err != nil {
						s.logger.Error("scheduled job failed",
							"job", sj.job.Name(),
							"error", err)
					}
				case <-sj.stop:
					return
				case <-ctx.Done():
					return
				}
			}
		}(sj)
	}
}

// Stop halts all scheduled jobs.
func (s *Scheduler) Stop() {
	for i := range s.jobs {
		if s.jobs[i].ticker != nil {
			s.jobs[i].ticker.Stop()
		}
		close(s.jobs[i].stop)
	}
	s.logger.Info("scheduler stopped")
}
