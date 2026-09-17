package retention_test

import (
	"context"
	"time"
)

// fakePruner implementiert report.Pruner und merkt sich den zuletzt
// übergebenen cutoff — für Tests, die prüfen, dass ApplyNow den
// richtigen Zeitpunkt aus RetentionMonths berechnet.
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
