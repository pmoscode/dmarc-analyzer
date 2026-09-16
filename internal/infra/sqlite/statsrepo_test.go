package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// saveReportWithRecords speichert einen Report mit synthetischen Records:
// je Eintrag in specs ein Record mit gegebenem Count, Disposition und
// aligned DKIM/SPF-Ergebnis.
type recordSpec struct {
	count       int
	disposition report.Disposition
	dkim, spf   report.AuthResultValue
	sourceIP    string
}

func saveReportWithRecords(t testing.TB, repo *sqlite.ReportRepository, reportID string, begin time.Time, specs []recordSpec) {
	t.Helper()
	ctx := context.Background()

	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	dateRange, err := report.NewDateRange(begin, begin.Add(time.Hour))
	require.NoError(t, err)
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	require.NoError(t, err)
	headerFrom, err := report.NewDomainName("example.com")
	require.NoError(t, err)

	records := make([]report.Record, 0, len(specs))
	for i, spec := range specs {
		ip := spec.sourceIP
		if ip == "" {
			ip = "203.0.113.1"
		}
		sourceIP, err := report.NewSourceIP(ip)
		require.NoError(t, err)

		rec, err := report.NewRecord(
			sourceIP, spec.count,
			report.PolicyEvaluation{Disposition: spec.disposition, DKIM: spec.dkim, SPF: spec.spf},
			report.Identifiers{HeaderFrom: headerFrom},
			report.AuthResults{},
		)
		require.NoError(t, err, "record #%d", i)
		records = append(records, rec)
	}

	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "stats-org.example", ReportID: reportID, Range: dateRange},
		policy, records, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, r))
}

func TestStatisticsRepository_Compute_BasicRates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "stats-1", begin, []recordSpec{
		{count: 10, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.1"},
		{count: 5, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultFail, sourceIP: "203.0.113.2"},
		{count: 3, disposition: report.DispositionReject, dkim: report.AuthResultFail, spf: report.AuthResultPass, sourceIP: "203.0.113.3"},
		{count: 2, disposition: report.DispositionReject, dkim: report.AuthResultFail, spf: report.AuthResultFail, sourceIP: "203.0.113.1"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	got, err := stats.Compute(ctx, analysis.Query{Period: period})
	require.NoError(t, err)

	require.Equal(t, 20, got.TotalMessages) // 10+5+3+2
	// pass: dkim=pass ODER spf=pass → records 1,2,3 zählen (10+5+3=18), record 4 nicht (2)
	require.InDelta(t, 18.0/20.0, got.PassRate, 0.0001)
	// dkim=pass: records 1,2 → 15
	require.InDelta(t, 15.0/20.0, got.DKIMAlignmentRate, 0.0001)
	// spf=pass: records 1,3 → 13
	require.InDelta(t, 13.0/20.0, got.SPFAlignmentRate, 0.0001)
	require.Equal(t, 3, got.DistinctSources) // .1, .2, .3

	require.Equal(t, 15, got.VolumeByDisposition[report.DispositionNone])  // 10+5
	require.Equal(t, 5, got.VolumeByDisposition[report.DispositionReject]) // 3+2
}

func TestStatisticsRepository_Compute_EmptyPeriod_NoDivisionByZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stats := sqlite.NewStatisticsRepository(newTestDB(t))

	period, err := report.NewDateRange(time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)

	got, err := stats.Compute(ctx, analysis.Query{Period: period})
	require.NoError(t, err)
	require.Zero(t, got.TotalMessages)
	require.Zero(t, got.PassRate)
	require.Zero(t, got.DKIMAlignmentRate)
	require.Zero(t, got.SPFAlignmentRate)
	require.Zero(t, got.DistinctSources)
	require.Empty(t, got.VolumeByDisposition)
}

func TestStatisticsRepository_Compute_FiltersByPeriod(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	saveReportWithRecords(t, reports, "jan-report", jan, []recordSpec{
		{count: 100, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
	})
	saveReportWithRecords(t, reports, "feb-report", feb, []recordSpec{
		{count: 7, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
	})

	period, err := report.NewDateRange(feb, feb.Add(time.Hour))
	require.NoError(t, err)

	got, err := stats.Compute(ctx, analysis.Query{Period: period})
	require.NoError(t, err)
	require.Equal(t, 7, got.TotalMessages, "Januar-Report liegt außerhalb des Zeitraums")
}

func TestStatisticsRepository_Compute_FiltersByDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "domain-report", begin, []recordSpec{
		{count: 42, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	gotMatch, err := stats.Compute(ctx, analysis.Query{Period: period, Domain: "example.com"})
	require.NoError(t, err)
	require.Equal(t, 42, gotMatch.TotalMessages)

	gotNoMatch, err := stats.Compute(ctx, analysis.Query{Period: period, Domain: "andere-domain.example"})
	require.NoError(t, err)
	require.Zero(t, gotNoMatch.TotalMessages)
}
