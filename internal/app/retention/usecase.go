// Package retention implements the retention policy (work package 7,
// IMPLEMENTIERUNG.md O-7: "default retention period for reports? 24
// months, changeable in settings.") — the period has come from
// internal/infra/envconfig ever since the switch to pure ENV
// configuration, no longer from a setting changeable at runtime.
package retention

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// UseCase applies the retention policy. Reports is deliberately of type
// report.Pruner instead of report.Repository — this use case only needs
// DeleteOlderThan, not full repository access (see comment on
// report.Pruner).
type UseCase struct {
	// RetentionMonths comes from envconfig.Config.RetentionMonths — 0
	// means unlimited retention, no automatic deletion.
	RetentionMonths int
	Reports         report.Pruner

	// Now returns the current point in time — nil uses time.Now.
	// Swappable for tests with a fixed point in time.
	Now func() time.Time
}

func (u *UseCase) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

// ApplyNow deletes all reports considered too old per the configured
// retention period, and returns the number of deleted reports.
// RetentionMonths <= 0 (unlimited retention) deletes nothing.
func (u *UseCase) ApplyNow(ctx context.Context) (int64, error) {
	if u.RetentionMonths <= 0 {
		return 0, nil
	}

	cutoff := u.now().AddDate(0, -u.RetentionMonths, 0)
	return u.Reports.DeleteOlderThan(ctx, cutoff)
}
