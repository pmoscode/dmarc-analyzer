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

// TestPerformance_FailedRecordsQuery_LargeVolume deckt AGENTS.md ab ("bei
// neuen Batch-Ladefunktionen ... immer mit realistisch großen
// Datenmengen testen") für die neue, berichtsübergreifende
// Fehlschläge-Abfrage: viele überwiegend bestehende Records (Rauschen),
// dazwischen verstreut eine kleine, bekannte Menge fehlgeschlagener
// Records — dieselbe Größenordnung wie
// TestPerformance_100kRecords_ImportAndQuery (reportperf_test.go).
func TestPerformance_FailedRecordsQuery_LargeVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("importiert 100.000 Records, siehe task test:unit")
	}
	t.Parallel()
	ctx := context.Background()

	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	failed := sqlite.NewFailedRecordsRepository(db)

	const (
		reportsCount     = 1000
		recordsPerReport = 100 // 1000 * 100 = 100.000 überwiegend bestehende Records
		failingReports   = 60  // gezielt verstreute Fehlschläge, mehr als eine Seite (50)
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
	t.Logf("Import von %d überwiegend bestehenden Records + %d gezielt fehlgeschlagenen Records: %s",
		reportsCount*recordsPerReport, failingReports, importDuration)

	// Bewusst OHNE Zeitraum-Filter — das ist der ungünstigste Fall (kein
	// idx_reports_period, der die zu prüfenden Records vorab eingrenzt),
	// die Abfrage muss dkim_result/spf_result über praktisch den gesamten
	// Bestand auswerten.
	queryStart := time.Now()
	page, err := failed.Query(ctx, failedrecords.Query{SortField: failedrecords.SortByDate, Limit: 50})
	queryDuration := time.Since(queryStart)
	require.NoError(t, err)
	require.Len(t, page.Records, 50)
	require.NotEmpty(t, page.NextCursor)
	t.Logf("Query (Fehlschläge-Ansicht, 50 von %d fehlgeschlagenen Records, ohne Zeitraum-Filter über ~100.000 Records gesamt): %s",
		failingReports, queryDuration)
	require.Less(t, queryDuration, 500*time.Millisecond,
		"Query der Fehlschläge-Ansicht ohne Zeitraum-Filter über einen Bestand von ~100.000 Records sollte trotz "+
			"fehlendem Index auf (dkim_result, spf_result) in vertretbarer Zeit laufen; bleibt das nicht der Fall, "+
			"Migration mit einem solchen Index nachziehen (siehe Kommentar in failedrecordsrepo.go)")
}
