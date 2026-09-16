package report_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func validMetadata(t *testing.T) report.Metadata {
	t.Helper()

	dr, err := report.NewDateRange(
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	return report.Metadata{
		OrgName:  "google.com",
		ReportID: "9391651994964116463",
		Range:    dr,
	}
}

func TestNewAggregateReport_RequiresReportID(t *testing.T) {
	t.Parallel()

	metadata := validMetadata(t)
	metadata.ReportID = "   "

	_, err := report.NewAggregateReport(metadata, report.PublishedPolicy{}, nil, report.SourceReference{}, time.Now())
	require.Error(t, err)
}

func TestNewAggregateReport_RequiresValidDateRange(t *testing.T) {
	t.Parallel()

	metadata := validMetadata(t)
	metadata.Range = report.DateRange{}

	_, err := report.NewAggregateReport(metadata, report.PublishedPolicy{}, nil, report.SourceReference{}, time.Now())
	require.Error(t, err)
}

func TestNewAggregateReport_ZeroRecordsIsValid(t *testing.T) {
	t.Parallel()

	// Ein Report ohne Records ist fachlich zulässig — z. B. wenn im
	// Zeitraum keine Nachrichten der berichtenden Domain zugestellt wurden
	// (siehe Testfall in IMPLEMENTIERUNG.md Abschnitt 12.3).
	r, err := report.NewAggregateReport(validMetadata(t), report.PublishedPolicy{}, nil, report.SourceReference{}, time.Now())

	require.NoError(t, err)
	require.Empty(t, r.Records)
}

func TestAggregateReport_Key(t *testing.T) {
	t.Parallel()

	metadata := validMetadata(t)
	r, err := report.NewAggregateReport(metadata, report.PublishedPolicy{}, nil, report.SourceReference{}, time.Now())
	require.NoError(t, err)

	key := r.Key()
	require.Equal(t, metadata.OrgName, key.OrgName)
	require.Equal(t, metadata.ReportID, key.ReportID)
	require.True(t, key.DateBegin.Equal(metadata.Range.Begin))
}

func TestNewAggregateReport_ImportedAtIsNormalizedToUTC(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	localTime := time.Date(2026, 9, 1, 8, 0, 0, 0, loc)

	r, err := report.NewAggregateReport(validMetadata(t), report.PublishedPolicy{}, nil, report.SourceReference{}, localTime)
	require.NoError(t, err)

	require.Equal(t, time.UTC, r.ImportedAt.Location())
	require.True(t, r.ImportedAt.Equal(localTime))
}
