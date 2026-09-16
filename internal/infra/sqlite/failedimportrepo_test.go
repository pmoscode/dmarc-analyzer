package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestFailedImportRepository_Record(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	repo := sqlite.NewFailedImportRepository(db)

	err := repo.Record(ctx, domainsync.FailedImport{
		AccountID:  "acc-1",
		MessageUID: 42,
		Filename:   "report.xml.gz",
		Error:      "gzip konnte nicht geöffnet werden",
		Raw:        []byte("kaputte-daten"),
		OccurredAt: time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM failed_imports").Scan(&count))
	require.Equal(t, 1, count)

	var filename, errMsg string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT filename, error FROM failed_imports").Scan(&filename, &errMsg))
	require.Equal(t, "report.xml.gz", filename)
	require.Equal(t, "gzip konnte nicht geöffnet werden", errMsg)
}

func TestFailedImportRepository_Record_WithoutTimestamp_DefaultsToNow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	repo := sqlite.NewFailedImportRepository(db)

	before := time.Now().Add(-time.Second)
	require.NoError(t, repo.Record(ctx, domainsync.FailedImport{Error: "x"}))
	after := time.Now().Add(time.Second)

	var occurredAt string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT occurred_at FROM failed_imports").Scan(&occurredAt))
	parsed, err := time.Parse(time.RFC3339Nano, occurredAt)
	require.NoError(t, err)
	require.True(t, parsed.After(before) && parsed.Before(after))
}

func TestFailedImportRepository_Record_MultipleEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	repo := sqlite.NewFailedImportRepository(db)

	require.NoError(t, repo.Record(ctx, domainsync.FailedImport{Filename: "a.xml", Error: "1"}))
	require.NoError(t, repo.Record(ctx, domainsync.FailedImport{Filename: "b.xml", Error: "2"}))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM failed_imports").Scan(&count))
	require.Equal(t, 2, count)
}
