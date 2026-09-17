package retention_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
)

func TestApplyNow_ZeroRetention_DoesNotDelete(t *testing.T) {
	pruner := &fakePruner{deleted: 5}
	u := &retention.UseCase{RetentionMonths: 0, Reports: pruner}

	deleted, err := u.ApplyNow(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
	require.Equal(t, 0, pruner.calls, "bei unbegrenzter Aufbewahrung darf nicht gelöscht werden")
}

func TestApplyNow_NegativeRetention_DoesNotDelete(t *testing.T) {
	pruner := &fakePruner{deleted: 5}
	u := &retention.UseCase{RetentionMonths: -1, Reports: pruner}

	deleted, err := u.ApplyNow(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
	require.Equal(t, 0, pruner.calls)
}

func TestApplyNow_ComputesCutoffFromRetentionMonths(t *testing.T) {
	fixedNow := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	pruner := &fakePruner{deleted: 3}
	u := &retention.UseCase{RetentionMonths: 24, Reports: pruner, Now: func() time.Time { return fixedNow }}

	deleted, err := u.ApplyNow(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.Equal(t, fixedNow.AddDate(0, -24, 0), pruner.lastCutoff)
}

func TestApplyNow_PropagatesDeleteError(t *testing.T) {
	pruner := &fakePruner{err: errTest}
	u := &retention.UseCase{RetentionMonths: 24, Reports: pruner}

	_, err := u.ApplyNow(context.Background())

	require.ErrorIs(t, err, errTest)
}

var errTest = errors.New("testfehler")
