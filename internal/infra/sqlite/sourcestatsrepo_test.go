package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestSourceStatsRepository_Query_AggregatesAcrossReports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewSourceStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "src-1", begin, []recordSpec{
		{count: 10, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.1"},
	})
	saveReportWithRecords(t, reports, "src-2", begin.Add(time.Hour), []recordSpec{
		{count: 5, disposition: report.DispositionNone, dkim: report.AuthResultFail, spf: report.AuthResultFail, sourceIP: "203.0.113.1"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(3*time.Hour))
	require.NoError(t, err)

	page, err := stats.Query(ctx, sources.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Stats, 1, "dieselbe Quell-IP über zwei Reports muss zu einer Zeile aggregiert werden")

	got := page.Stats[0]
	require.Equal(t, "203.0.113.1", got.SourceIP.String())
	require.Equal(t, 15, got.TotalCount)
	require.InDelta(t, 10.0/15.0, got.PassRate, 0.0001)
	require.InDelta(t, 10.0/15.0, got.DKIMPassRate, 0.0001, "nur src-1 (count 10) hat dkim=pass")
	require.InDelta(t, 10.0/15.0, got.SPFPassRate, 0.0001, "nur src-1 (count 10) hat spf=pass")
	require.Empty(t, page.NextCursor)
}

func TestSourceStatsRepository_Query_SeparatesDKIMAndSPFPassRate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewSourceStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	// Typisches Weiterleitungs-Muster: DKIM übersteht die Weiterleitung
	// (besteht), SPF bricht (die weiterleitende IP steht nicht im
	// SPF-Record der ursprünglichen Domain) — DMARC besteht trotzdem
	// (PassRate zählt dkim ODER spf), DKIMPassRate und SPFPassRate
	// müssen das aber unterscheidbar machen.
	saveReportWithRecords(t, reports, "forward-1", begin, []recordSpec{
		{count: 20, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultFail, sourceIP: "203.0.113.7"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	page, err := stats.Query(ctx, sources.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Stats, 1)

	got := page.Stats[0]
	require.InDelta(t, 1.0, got.PassRate, 0.0001, "dkim=pass reicht für DMARC")
	require.InDelta(t, 1.0, got.DKIMPassRate, 0.0001)
	require.InDelta(t, 0.0, got.SPFPassRate, 0.0001)
}

func TestSourceStatsRepository_Query_SortByVolume_PaginatesWithCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewSourceStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "page-1", begin, []recordSpec{
		{count: 30, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.1"},
		{count: 20, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.2"},
		{count: 10, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.3"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	first, err := stats.Query(ctx, sources.Query{Period: &period, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Stats, 2)
	require.Equal(t, "203.0.113.1", first.Stats[0].SourceIP.String())
	require.Equal(t, "203.0.113.2", first.Stats[1].SourceIP.String())
	require.NotEmpty(t, first.NextCursor)

	second, err := stats.Query(ctx, sources.Query{Period: &period, Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Stats, 1)
	require.Equal(t, "203.0.113.3", second.Stats[0].SourceIP.String())
	require.Empty(t, second.NextCursor)
}

func TestSourceStatsRepository_Query_SortByIP_Ascending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewSourceStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "sort-ip", begin, []recordSpec{
		{count: 1, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.9"},
		{count: 1, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.2"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	page, err := stats.Query(ctx, sources.Query{Period: &period, SortField: sources.SortByIP})
	require.NoError(t, err)
	require.Len(t, page.Stats, 2)
	require.Equal(t, "203.0.113.2", page.Stats[0].SourceIP.String())
	require.Equal(t, "203.0.113.9", page.Stats[1].SourceIP.String())
}

func TestSourceStatsRepository_Query_FiltersByDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewSourceStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "domain-filter", begin, []recordSpec{
		{count: 42, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	match, err := stats.Query(ctx, sources.Query{Period: &period, Domain: "example.com"})
	require.NoError(t, err)
	require.Len(t, match.Stats, 1)

	noMatch, err := stats.Query(ctx, sources.Query{Period: &period, Domain: "andere-domain.example"})
	require.NoError(t, err)
	require.Empty(t, noMatch.Stats)
}
