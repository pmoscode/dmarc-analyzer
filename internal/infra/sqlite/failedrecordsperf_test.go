package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// TestPerformance_FailedRecordsQuery_LargeVolume covers AGENTS.md ("for new
// batch-loading functions ... always test with realistically large data
// volumes") for the new, cross-report failed-records query: many mostly
// passing records (noise), with a small, known set of failed records
// scattered among them — the same order of magnitude as
// TestPerformance_100kRecords_ImportAndQuery (reportperf_test.go).
func TestPerformance_FailedRecordsQuery_LargeVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("imports 100,000 records, see task test:unit")
	}
	t.Parallel()
	ctx := context.Background()

	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	const (
		reportsCount     = 1000
		recordsPerReport = 100 // 1000 * 100 = 100,000 mostly passing records
		failingReports   = 60  // deliberately scattered failures, more than one page (50)
	)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	importStart := time.Now()
	for i := 0; i < reportsCount; i++ {
		r := newLargeTestReport(t, fmt.Sprintf("failedperf-%05d", i), base.Add(time.Duration(i)*time.Minute), recordsPerReport)
		require.NoError(t, reports.Save(ctx, r))
	}
	for i := 0; i < failingReports; i++ {
		begin := base.Add(24 * time.Hour).Add(time.Duration(i) * time.Minute)
		ip, err := report.NewSourceIP(fmt.Sprintf("198.51.100.%d", i+1))
		require.NoError(t, err)
		domain, err := report.NewDomainName("perf.example")
		require.NoError(t, err)
		rec, err := report.NewRecord(ip, 1,
			report.PolicyEvaluation{Disposition: report.DispositionReject, DKIM: report.AuthResultFail, SPF: report.AuthResultFail},
			report.Identifiers{HeaderFrom: domain}, report.AuthResults{})
		require.NoError(t, err)

		dateRange, err := report.NewDateRange(begin, begin.Add(time.Hour))
		require.NoError(t, err)
		policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject,
			report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
		require.NoError(t, err)
		r, err := report.NewAggregateReport(
			report.Metadata{OrgName: "perf-org.example", ReportID: fmt.Sprintf("failedperf-fail-%03d", i), Range: dateRange},
			policy, []report.Record{rec}, report.SourceReference{}, time.Now(),
		)
		require.NoError(t, err)
		require.NoError(t, reports.Save(ctx, r))
	}
	importDuration := time.Since(importStart)
	t.Logf("Import of %d mostly passing records + %d deliberately failed records: %s",
		reportsCount*recordsPerReport, failingReports, importDuration)

	// Deliberately WITHOUT a period filter — that's the worst case (no
	// idx_reports_period to narrow down the records to check
	// beforehand), the query has to evaluate dkim_result/spf_result over
	// practically the entire dataset.
	queryStart := time.Now()
	page, err := failed.Query(ctx, failedrecords.Query{SortField: failedrecords.SortByDate, Limit: 50})
	queryDuration := time.Since(queryStart)
	require.NoError(t, err)
	require.Len(t, page.Records, 50)
	require.NotEmpty(t, page.NextCursor)
	t.Logf("Query (failed-records view, 50 of %d failed records, no period filter over ~100,000 records total): %s",
		failingReports, queryDuration)
	require.Less(t, queryDuration, 500*time.Millisecond,
		"querying the failed-records view without a period filter over a dataset of ~100,000 records should "+
			"complete in a reasonable time despite the missing index on (dkim_result, spf_result); if this stops "+
			"holding, add a migration with such an index (see the comment in failedrecordsrepo.go)")
}
