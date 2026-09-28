// Package retentionjob applies the retention policy (work package 7, see
// internal/app/retention) automatically in the background — once at
// startup and then at fixed intervals, analogous to internal/app/syncjob,
// just without a progress display: deleting old reports is an
// unobtrusive maintenance task, not an operation the user needs to
// observe or be able to cancel.
package retentionjob

import (
	"context"
	"log/slog"
	"time"
)

// Applier is the slice of retention.UseCase that Runner needs — its own
// interface instead of a concrete struct dependency, so tests can use a
// fake instead of a fully wired UseCase (the same idea as syncjob.Syncer).
type Applier interface {
	ApplyNow(ctx context.Context) (int64, error)
}

// Runner calls Applier.ApplyNow periodically until ctx ends.
type Runner struct {
	applier  Applier
	interval time.Duration
	logger   *slog.Logger
}

// NewRunner creates a Runner. logger == nil uses slog.Default().
func NewRunner(applier Applier, interval time.Duration, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{applier: applier, interval: interval, logger: logger}
}

// Run blocks until ctx ends — a first pass runs immediately at startup
// (a freshly changed retention period shouldn't only take effect after a
// full interval), then every interval.
func (r *Runner) Run(ctx context.Context) {
	r.apply(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.apply(ctx)
		}
	}
}

func (r *Runner) apply(ctx context.Context) {
	deleted, err := r.applier.ApplyNow(ctx)
	if err != nil {
		r.logger.Error("failed to apply retention policy", "error", err)
		return
	}
	if deleted > 0 {
		r.logger.Info("retention policy applied", "deleted_reports", deleted)
	}
}
