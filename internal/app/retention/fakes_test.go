package retention_test

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

// fakeSettingsRepository implementiert settings.Repository in-memory.
type fakeSettingsRepository struct {
	stored   settings.Settings
	hasValue bool

	loadErr error
	saveErr error
}

func (f *fakeSettingsRepository) Load(context.Context) (settings.Settings, error) {
	if f.loadErr != nil {
		return settings.Settings{}, f.loadErr
	}
	if !f.hasValue {
		return settings.Default(), nil
	}
	return f.stored, nil
}

func (f *fakeSettingsRepository) Save(_ context.Context, s settings.Settings) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.stored = s
	f.hasValue = true
	return nil
}

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
