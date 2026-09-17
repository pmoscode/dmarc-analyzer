package retention_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

func TestLoadSettings_DelegatesToRepository(t *testing.T) {
	repo := &fakeSettingsRepository{stored: settings.Settings{RetentionMonths: 6, SyncIntervalMinutes: 15}, hasValue: true}
	u := &retention.UseCase{Settings: repo}

	got, err := u.LoadSettings(context.Background())

	require.NoError(t, err)
	require.Equal(t, repo.stored, got)
}

func TestSaveSettings_RejectsInvalidValues(t *testing.T) {
	repo := &fakeSettingsRepository{}
	u := &retention.UseCase{Settings: repo}

	err := u.SaveSettings(context.Background(), settings.Settings{RetentionMonths: -1})

	require.Error(t, err)
	require.False(t, repo.hasValue, "ungültige Einstellungen dürfen nicht gespeichert werden")
}

func TestSaveSettings_PersistsValidValues(t *testing.T) {
	repo := &fakeSettingsRepository{}
	u := &retention.UseCase{Settings: repo}
	want := settings.Settings{RetentionMonths: 12, SyncIntervalMinutes: 30}

	err := u.SaveSettings(context.Background(), want)

	require.NoError(t, err)
	require.Equal(t, want, repo.stored)
}

func TestApplyNow_ZeroRetention_DoesNotDelete(t *testing.T) {
	repo := &fakeSettingsRepository{stored: settings.Settings{RetentionMonths: 0}, hasValue: true}
	pruner := &fakePruner{deleted: 5}
	u := &retention.UseCase{Settings: repo, Reports: pruner}

	deleted, err := u.ApplyNow(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
	require.Equal(t, 0, pruner.calls, "bei unbegrenzter Aufbewahrung darf nicht gelöscht werden")
}

func TestApplyNow_ComputesCutoffFromRetentionMonths(t *testing.T) {
	fixedNow := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	repo := &fakeSettingsRepository{stored: settings.Settings{RetentionMonths: 24}, hasValue: true}
	pruner := &fakePruner{deleted: 3}
	u := &retention.UseCase{Settings: repo, Reports: pruner, Now: func() time.Time { return fixedNow }}

	deleted, err := u.ApplyNow(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.Equal(t, fixedNow.AddDate(0, -24, 0), pruner.lastCutoff)
}

func TestApplyNow_PropagatesSettingsLoadError(t *testing.T) {
	repo := &fakeSettingsRepository{loadErr: errTest}
	pruner := &fakePruner{}
	u := &retention.UseCase{Settings: repo, Reports: pruner}

	_, err := u.ApplyNow(context.Background())

	require.ErrorIs(t, err, errTest)
	require.Equal(t, 0, pruner.calls)
}

func TestApplyNow_PropagatesDeleteError(t *testing.T) {
	repo := &fakeSettingsRepository{stored: settings.Settings{RetentionMonths: 24}, hasValue: true}
	pruner := &fakePruner{err: errTest}
	u := &retention.UseCase{Settings: repo, Reports: pruner}

	_, err := u.ApplyNow(context.Background())

	require.ErrorIs(t, err, errTest)
}

type testErr string

func (e testErr) Error() string { return string(e) }

var errTest = testErr("testfehler")
