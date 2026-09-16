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

// newLargeTestReport baut einen Report mit n Records — für Import- und
// Abfrage-Performance-Tests. Records unterscheiden sich in der Quell-IP,
// damit Indizes wie im echten Betrieb genutzt werden statt auf identischen
// Werten zu operieren.
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

// TestPerformance_100kRecords_ImportAndQuery deckt den in
// UMSETZUNGSPLAN.md AP 2 geforderten Benchmark ab: 100.000 Records
// importieren und abfragen. Läuft als regulärer, unter -short
// übersprungener Test statt als go-test-Benchmark, weil das Szenario ein
// einmaliger Ablauf ist (Datenmenge aufbauen, dann eine realistische
// Abfrage messen), keine Mikro-Benchmark-Schleife. Zeitgrenzen bewusst mit
// Sicherheitsabstand zum Zielwert (100 ms) gewählt, um auf langsameren
// CI-Runnern nicht spontan zu flackern — die tatsächlich gemessenen Werte
// werden geloggt.
func TestPerformance_100kRecords_ImportAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("importiert 100.000 Records, siehe task test:unit")
	}
	t.Parallel()
	ctx := context.Background()

	repo := sqlite.NewReportRepository(newTestDB(t))

	const (
		reportsCount        = 1000
		recordsPerReport    = 100 // 1000 * 100 = 100.000 Records insgesamt
		bigReportRecordsCnt = 10000
	)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	importStart := time.Now()
	for i := 0; i < reportsCount; i++ {
		r := newLargeTestReport(t, fmt.Sprintf("perf-%05d", i), base.Add(time.Duration(i)*time.Minute), recordsPerReport)
		require.NoError(t, repo.Save(ctx, r))
	}
	// Ein einzelner großer Report zusätzlich — deckt den in
	// IMPLEMENTIERUNG.md Abschnitt 12.3 genannten Testfall "Report mit
	// 10.000 Records" ab und dient unten als FindByID-Ziel.
	bigReport := newLargeTestReport(t, "perf-big", base.Add(24*time.Hour), bigReportRecordsCnt)
	require.NoError(t, repo.Save(ctx, bigReport))
	importDuration := time.Since(importStart)
	t.Logf("Import von %d Reports (%d Records) + 1 Report mit %d Records: %s",
		reportsCount, reportsCount*recordsPerReport, bigReportRecordsCnt, importDuration)

	// Berichtstabelle: eine typische, gefilterte, sortierte, erste Seite.
	queryStart := time.Now()
	page, err := repo.Query(ctx, report.Query{
		SortField: report.SortByDateBegin, SortDirection: report.SortDescending, Limit: 50,
	})
	queryDuration := time.Since(queryStart)
	require.NoError(t, err)
	require.Len(t, page.Reports, 50)
	t.Logf("Query (Berichtstabelle, 50 von %d Reports): %s", reportsCount+1, queryDuration)
	require.Less(t, queryDuration, 100*time.Millisecond,
		"Query einer Seite der Berichtstabelle ist der eigentliche Zielwert aus UMSETZUNGSPLAN.md AP 2 — "+
			"lokal gemessen ~7-8ms, hier ohne künstlichen Sicherheitsabstand, weil der Wert das auch verdient")

	// Bericht-Detailansicht: den einen großen Report vollständig laden.
	// 10.000 Records in einem einzelnen Report ist deutlich mehr, als
	// reale DMARC-Aggregate-Reports üblicherweise enthalten (siehe
	// IMPLEMENTIERUNG.md Abschnitt 12.3, derselbe Testfall) — das ist der
	// Rand-/Lastfall, nicht der Alltagspfad, deshalb hier ein spürbar
	// großzügigerer Grenzwert als bei der Listenabfrage oben.
	findStart := time.Now()
	loaded, err := repo.FindByID(ctx, bigReport.ID)
	findDuration := time.Since(findStart)
	require.NoError(t, err)
	require.Len(t, loaded.Records, bigReportRecordsCnt)
	t.Logf("FindByID (1 Report, %d Records): %s", bigReportRecordsCnt, findDuration)
	require.Less(t, findDuration, 2*time.Second,
		"ein einzelner Report mit 10.000 Records (unrealistisch groß) sollte trotzdem in vertretbarer Zeit laden; "+
			"lokal gemessen ~460ms nach Umstellung der Batch-Ladefunktionen auf JOIN statt riesiger IN-Klausel")
}
