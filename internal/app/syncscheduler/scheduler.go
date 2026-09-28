// Package syncscheduler triggers automatic background syncs on a fixed
// schedule (work package 7: "scheduled background sync") — the run
// started manually via /abgleich (internal/app/syncjob) remains
// unaffected and still possible at any time. The interval comes from
// envconfig.Config.SyncIntervalMinutes and doesn't change at runtime
// (12-factor, no reload needed — unlike before the switch to pure ENV
// configuration, see this file's git history).
package syncscheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

// Starter is the slice of syncjob.Runner that Scheduler needs.
type Starter interface {
	Start() error
}

// Scheduler triggers a run after every interval, as long as none is
// running — if one is already running, syncjob.Runner.Start() reports
// ErrAlreadyRunning harmlessly, and the interval then lets the next
// attempt take effect automatically a bit later.
type Scheduler struct {
	job      Starter
	interval time.Duration
	logger   *slog.Logger
}

// NewScheduler creates a Scheduler. logger == nil uses slog.Default().
// interval <= 0 means: automatic sync disabled (Run() then returns
// immediately).
func NewScheduler(job Starter, interval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{job: job, interval: interval, logger: logger}
}

// Run blocks until ctx ends — a first run immediately at startup (a
// freshly started container shouldn't have to wait a full interval for
// the first sync), then every interval.
func (s *Scheduler) Run(ctx context.Context) {
	if s.interval <= 0 {
		return
	}

	s.trigger()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.trigger()
		}
	}
}

func (s *Scheduler) trigger() {
	err := s.job.Start()
	if err == nil || errors.Is(err, syncjob.ErrAlreadyRunning) {
		return
	}
	s.logger.Warn("scheduled sync could not be started", "error", err)
}
