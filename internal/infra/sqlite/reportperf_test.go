package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// newLargeTestReport builds a report with n records — for import and
// query performance tests. Records differ in source IP, so indexes are
// used as in real operation instead of operating on identical values.
func newLargeTestReport(t testing.TB, reportID string, begin time.Time, n int) *report.AggregateReport {
	t.Helper()

	domain, err := report.NewDomainName("perf.example")
	require.NoError(t, err)
	dateRange, err := report.NewDateRange(begin, begin.Add(time.Hour))
	require.NoError(t, err)
	policy, err := report.NewPublishedPolicy(
		domain, report.PolicyReject, report.PolicyReject,
		report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "",
	)
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName("perf.example")
	require.NoError(t, err)

	records := make([]report.Record, n)
	for i := 0; i < n; i++ {
		ip, err := report.NewSourceIP(fmt.Sprintf("10.%d.%d.%d", (i>>16)&0xff, (i>>8)&0xff, i&0xff))
		require.NoError(t, err)

		rec, err := report.NewRecord(
			ip, 1,
			report.PolicyEvaluation{Disposition: report.DispositionNone, DKIM: report.AuthResultPass, SPF: report.AuthResultPass},
			report.Identifiers{HeaderFrom: headerFrom},
			report.AuthResults{},
		)
		require.NoError(t, err)
		records[i] = rec
	}

	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "perf-org.example", ReportID: reportID, Range: dateRange},
		policy, records, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	return r
}

// TestPerformance_100kRecords_ImportAndQuery covers the benchmark required
// in UMSETZUNGSPLAN.md AP 2: import and query 100,000 records. Runs as a
// regular test skipped under -short instead of a go-test benchmark,
// because the scenario is a one-time run (build up a dataset, then measure
// one realistic query), not a micro-benchmark loop. Time limits are
// deliberately chosen with a safety margin over the target value (100 ms)
// so they don't flake spontaneously on slower CI runners — the actually
// measured values are logged.
func TestPerformance_100kRecords_ImportAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("imports 100,000 records, see task test:unit")
	}
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))

	const (
		reportsCount        = 1000
		recordsPerReport    = 100 // 1000 * 100 = 100,000 records total
		bigReportRecordsCnt = 10000
	)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	importStart := time.Now()
	for i := 0; i < reportsCount; i++ {
		r := newLargeTestReport(t, fmt.Sprintf("perf-%05d", i), base.Add(time.Duration(i)*time.Minute), recordsPerReport)
		require.NoError(t, repo.Save(ctx, r))
	}
	// One additional single large report — covers the test case "report
	// with 10,000 records" named in IMPLEMENTIERUNG.md section 12.3, and
	// serves as the FindByID target below.
	bigReport := newLargeTestReport(t, "perf-big", base.Add(24*time.Hour), bigReportRecordsCnt)
	require.NoError(t, repo.Save(ctx, bigReport))
	importDuration := time.Since(importStart)
	t.Logf("Import of %d reports (%d records) + 1 report with %d records: %s",
		reportsCount, reportsCount*recordsPerReport, bigReportRecordsCnt, importDuration)

	// Reports table: a typical, filtered, sorted first page.
	queryStart := time.Now()
	page, err := repo.Query(ctx, report.Query{
		SortField: report.SortByDateBegin, SortDirection: report.SortDescending, Limit: 50,
	})
	queryDuration := time.Since(queryStart)
	require.NoError(t, err)
	require.Len(t, page.Reports, 50)
	t.Logf("Query (reports table, 50 of %d reports): %s", reportsCount+1, queryDuration)
	require.Less(t, queryDuration, 100*time.Millisecond,
		"querying a page of the reports table is the actual target value from UMSETZUNGSPLAN.md AP 2 — "+
			"measured locally at ~7-8ms, here without an artificial safety margin because the value earns it")

	// Report detail view: fully load the one large report. 10,000 records
	// in a single report is considerably more than real DMARC aggregate
	// reports usually contain (see IMPLEMENTIERUNG.md section 12.3, same
	// test case) — this is the edge/load case, not the everyday path,
	// hence a noticeably more generous limit here than for the list query
	// above.
	findStart := time.Now()
	loaded, err := repo.FindByID(ctx, bigReport.ID)
	findDuration := time.Since(findStart)
	require.NoError(t, err)
	require.Len(t, loaded.Records, bigReportRecordsCnt)
	t.Logf("FindByID (1 report, %d records): %s", bigReportRecordsCnt, findDuration)
	require.Less(t, findDuration, 2*time.Second,
		"a single report with 10,000 records (unrealistically large) should still load in a reasonable time; "+
			"measured locally at ~460ms after switching the batch-loading functions to JOIN instead of a huge IN clause")
}
