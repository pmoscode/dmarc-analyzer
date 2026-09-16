package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestSave_And_FindByID_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))
	r := newTestReport(t, reportOpts{orgName: "roundtrip.example"})

	require.NoError(t, repo.Save(ctx, r))
	require.NotZero(t, r.ID, "Save muss die vergebene ID setzen")

	loaded, err := repo.FindByID(ctx, r.ID)
	require.NoError(t, err)

	require.Equal(t, r.Metadata.OrgName, loaded.Metadata.OrgName)
	require.Equal(t, r.Metadata.ReportID, loaded.Metadata.ReportID)
	require.True(t, r.Metadata.Range.Begin.Equal(loaded.Metadata.Range.Begin))
	require.True(t, r.Metadata.Range.End.Equal(loaded.Metadata.Range.End))
	require.Equal(t, r.Policy.Domain.String(), loaded.Policy.Domain.String())
	require.Equal(t, r.Policy.Percentage, loaded.Policy.Percentage)

	require.Len(t, loaded.Records, 1)
	require.Equal(t, r.Records[0].SourceIP.String(), loaded.Records[0].SourceIP.String())
	require.Equal(t, r.Records[0].Count, loaded.Records[0].Count)
	require.Equal(t, r.Records[0].Evaluated.Disposition, loaded.Records[0].Evaluated.Disposition)
	require.Len(t, loaded.Records[0].Evaluated.Reasons, 1)
	require.Equal(t, "local_policy", loaded.Records[0].Evaluated.Reasons[0].Type)
	require.Len(t, loaded.Records[0].Auth.DKIM, 1)
	require.Equal(t, "sel1", loaded.Records[0].Auth.DKIM[0].Selector)
	require.Len(t, loaded.Records[0].Auth.SPF, 1)
}

func TestSave_ReportWithoutRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))
	r := newTestReport(t, reportOpts{})
	r.Records = nil

	require.NoError(t, repo.Save(ctx, r))

	loaded, err := repo.FindByID(ctx, r.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.Records)
}

func TestSave_ReportWithErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))
	r := newTestReport(t, reportOpts{})
	r.Metadata.Errors = []string{"unbekanntes element im xml", "pct fehlte"}

	require.NoError(t, repo.Save(ctx, r))

	loaded, err := repo.FindByID(ctx, r.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, r.Metadata.Errors, loaded.Metadata.Errors)
}

func TestSave_DuplicateKey_ReturnsErrDuplicateReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))

	first := newTestReport(t, reportOpts{orgName: "dup.example", reportID: "dup-1"})
	require.NoError(t, repo.Save(ctx, first))

	// Zweiter Report mit identischer fachlicher Identität
	// (org_name, report_id, date_begin) — z. B. dieselbe Mail zweimal
	// abgeholt (IMPLEMENTIERUNG.md Abschnitt 6.3).
	second := newTestReport(t, reportOpts{orgName: "dup.example", reportID: "dup-1"})

	err := repo.Save(ctx, second)
	require.ErrorIs(t, err, report.ErrDuplicate)
}

func TestExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))
	r := newTestReport(t, reportOpts{orgName: "exists.example", reportID: "exists-1"})

	exists, err := repo.Exists(ctx, r.Key())
	require.NoError(t, err)
	require.False(t, exists, "vor dem Speichern darf der Report nicht existieren")

	require.NoError(t, repo.Save(ctx, r))

	exists, err = repo.Exists(ctx, r.Key())
	require.NoError(t, err)
	require.True(t, exists, "nach dem Speichern muss der Report existieren")
}

func TestFindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))

	_, err := repo.FindByID(ctx, report.ReportID(999999))
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestSave_ContextCancelled_RollsBackFully(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repo := sqlite.NewReportRepository(db)
	r := newTestReport(t, reportOpts{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.Save(ctx, r)
	require.Error(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM reports").Scan(&count))
	require.Zero(t, count, "bei abgebrochenem Kontext darf kein Report übrig bleiben")
}

func TestSave_SetsIDOnlyOnSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))
	r := newTestReport(t, reportOpts{orgName: "id-on-success.example", reportID: "id-1"})

	require.NoError(t, repo.Save(ctx, r))
	firstID := r.ID
	require.NotZero(t, firstID)

	// Erneutes Speichern desselben Domänenobjekts (gleiche Identität)
	// scheitert an der Deduplizierung — r.ID darf sich dabei nicht ändern.
	err := repo.Save(ctx, r)
	require.True(t, errors.Is(err, report.ErrDuplicate))
	require.Equal(t, firstID, r.ID)
}

func TestFindByID_CorruptedMessageUID_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := newTestDB(t)
	repo := sqlite.NewReportRepository(db)
	r := newTestReport(t, reportOpts{orgName: "corrupt.example", reportID: "corrupt-1"})
	require.NoError(t, repo.Save(ctx, r))

	// Simuliert eine von außerhalb dieses Programms manipulierte Datei:
	// message_uid außerhalb des uint32-Bereichs (IMAP-UIDs sind 32 Bit).
	_, err := db.ExecContext(ctx, "UPDATE reports SET message_uid = ? WHERE id = ?", int64(1)<<40, int64(r.ID))
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, r.ID)
	require.Error(t, err, "ein message_uid außerhalb des uint32-Bereichs muss abgelehnt werden, nicht still abgeschnitten")
}
