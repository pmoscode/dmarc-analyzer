package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestFailedRecordsRepository_Query_OnlyReturnsFailingRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "ok-1", begin: begin, sourceIP: "203.0.113.1"})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "fail-1", begin: begin.Add(time.Hour), sourceIP: "198.51.100.1",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	// DKIM allein bestanden -> PassesDMARC() true, gehört NICHT zu den
	// Fehlschlägen (dieselbe Definition wie recordDetailView.PassesDMARC/
	// AGENTS.md-Plan: "fehlgeschlagen" = weder DKIM noch SPF aligned).
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "partial-1", begin: begin.Add(2 * time.Hour), sourceIP: "198.51.100.2",
		dkim: report.AuthResultPass, spf: report.AuthResultFail})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(3*time.Hour))
	require.NoError(t, err)

	page, err := failed.Query(ctx, failedrecords.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	require.Equal(t, "198.51.100.1", page.Records[0].SourceIP.String())
	require.Equal(t, report.AuthResultFail, page.Records[0].DKIM)
	require.Equal(t, report.AuthResultFail, page.Records[0].SPF)
	require.Equal(t, "fail-1", func() string {
		full, err := reports.FindByID(ctx, page.Records[0].ReportID)
		require.NoError(t, err)
		return full.Metadata.ReportID
	}())
}

func TestFailedRecordsRepository_Query_IncludesRawAuthResultsAndReasons(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "raw-1", begin: begin, sourceIP: "198.51.100.1",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(time.Hour))
	require.NoError(t, err)

	page, err := failed.Query(ctx, failedrecords.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Records, 1)

	rec := page.Records[0]
	require.Len(t, rec.Auth.DKIM, 1)
	require.Equal(t, "sel1", rec.Auth.DKIM[0].Selector)
	require.Len(t, rec.Auth.SPF, 1)
	require.Len(t, rec.Reasons, 1)
	require.Equal(t, "local_policy", rec.Reasons[0].Type)
	require.Equal(t, "example.com", rec.PolicyDomain.String())
	require.Equal(t, "test-org.example", rec.OrgName)
}

func TestFailedRecordsRepository_Query_FiltersByDomainAndSourceIP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "match.example", reportID: "filter-match", begin: begin, sourceIP: "198.51.100.1",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	saveTestReport(t, reports, reportOpts{domain: "other.example", reportID: "filter-other", begin: begin, sourceIP: "198.51.100.2",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(time.Hour))
	require.NoError(t, err)

	byDomain, err := failed.Query(ctx, failedrecords.Query{Period: &period, Domain: "match.example"})
	require.NoError(t, err)
	require.Len(t, byDomain.Records, 1)
	require.Equal(t, "match.example", byDomain.Records[0].PolicyDomain.String())

	bySourceIP, err := failed.Query(ctx, failedrecords.Query{Period: &period, SourceIP: "198.51.100.2"})
	require.NoError(t, err)
	require.Len(t, bySourceIP.Records, 1)
	require.Equal(t, "198.51.100.2", bySourceIP.Records[0].SourceIP.String())
}

func TestFailedRecordsRepository_Query_SortByDate_PaginatesWithCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "date-1", begin: begin, sourceIP: "198.51.100.1",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "date-2", begin: begin.Add(time.Hour), sourceIP: "198.51.100.2",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "date-3", begin: begin.Add(2 * time.Hour), sourceIP: "198.51.100.3",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(3*time.Hour))
	require.NoError(t, err)

	first, err := failed.Query(ctx, failedrecords.Query{Period: &period, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Records, 2)
	require.Equal(t, "198.51.100.3", first.Records[0].SourceIP.String(), "SortByDate: neuester Bericht zuerst")
	require.Equal(t, "198.51.100.2", first.Records[1].SourceIP.String())
	require.NotEmpty(t, first.NextCursor)

	second, err := failed.Query(ctx, failedrecords.Query{Period: &period, Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Records, 1)
	require.Equal(t, "198.51.100.1", second.Records[0].SourceIP.String())
	require.Empty(t, second.NextCursor)
}

func TestFailedRecordsRepository_Query_SortBySourceIP_PaginatesWithCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "ip-1", begin: begin, sourceIP: "198.51.100.3",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "ip-2", begin: begin, sourceIP: "198.51.100.1",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "ip-3", begin: begin, sourceIP: "198.51.100.2",
		dkim: report.AuthResultFail, spf: report.AuthResultFail})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(time.Hour))
	require.NoError(t, err)

	first, err := failed.Query(ctx, failedrecords.Query{Period: &period, SortField: failedrecords.SortBySourceIP, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Records, 2)
	require.Equal(t, "198.51.100.1", first.Records[0].SourceIP.String())
	require.Equal(t, "198.51.100.2", first.Records[1].SourceIP.String())
	require.NotEmpty(t, first.NextCursor)

	second, err := failed.Query(ctx, failedrecords.Query{Period: &period, SortField: failedrecords.SortBySourceIP, Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Records, 1)
	require.Equal(t, "198.51.100.3", second.Records[0].SourceIP.String())
	require.Empty(t, second.NextCursor)
}
