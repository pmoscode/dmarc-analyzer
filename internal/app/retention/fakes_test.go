package retention_test

import (
	"context"
	"time"
)

// fakePruner implements report.Pruner and remembers the last cutoff it
// was given — for tests that verify ApplyNow computes the correct point
// in time from RetentionMonths.
type fakePruner struct {
	deleted    int64
	err        error
	lastCutoff time.Time
	calls      int
}

func (f *fakePruner) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.calls++
	f.lastCutoff = cutoff
	if f.err != nil {
		return 0, f.err
	}
	return f.deleted, nil
}
