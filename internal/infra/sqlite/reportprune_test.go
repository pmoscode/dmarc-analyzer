package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestDeleteOlderThan_DeletesOnlyReportsEndingBeforeCutoff(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewReportRepository(db)
	ctx := context.Background()

	old := newTestReport(t, reportOpts{
		reportID: "old",
		begin:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		end:      time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	recent := newTestReport(t, reportOpts{
		reportID: "recent",
		begin:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		end:      time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, repo.Save(ctx, old))
	require.NoError(t, repo.Save(ctx, recent))

	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	_, err = repo.FindByID(ctx, old.ID)
	require.Error(t, err)

	stillThere, err := repo.FindByID(ctx, recent.ID)
	require.NoError(t, err)
	require.Equal(t, recent.ID, stillThere.ID)
}

func TestDeleteOlderThan_CascadesToRecords(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewReportRepository(db)
	ctx := context.Background()

	old := newTestReport(t, reportOpts{
		reportID: "old-with-records",
		begin:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		end:      time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, repo.Save(ctx, old))

	deleted, err := repo.DeleteOlderThan(ctx, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	var recordCount int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM records WHERE report_id = ?", old.ID).Scan(&recordCount))
	require.Equal(t, 0, recordCount)
}

func TestDeleteOlderThan_NothingToDelete_ReturnsZero(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewReportRepository(db)
	ctx := context.Background()

	recent := newTestReport(t, reportOpts{})
	require.NoError(t, repo.Save(ctx, recent))

	deleted, err := repo.DeleteOlderThan(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
}
