package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestDomainStatsRepository_Query_AggregatesAcrossReports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewDomainStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "dom-1", begin: begin, end: begin.Add(time.Hour), sourceIP: "203.0.113.1"})
	saveTestReport(t, reports, reportOpts{domain: "example.com", reportID: "dom-2", begin: begin.Add(time.Hour), end: begin.Add(2 * time.Hour), sourceIP: "203.0.113.2"})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(3*time.Hour))
	require.NoError(t, err)

	page, err := stats.Query(ctx, domainstats.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Stats, 1, "zwei Reports derselben Domain müssen zu einer Zeile aggregiert werden")

	got := page.Stats[0]
	require.Equal(t, "example.com", got.Domain.String())
	require.Equal(t, 6, got.TotalCount, "je Report ein Record mit message_count 3 (siehe newTestReport)")
	require.Equal(t, 2, got.ReportCount)
	require.Equal(t, 2, got.DistinctSources)
	require.InDelta(t, 1.0, got.PassRate, 0.0001, "newTestReport erzeugt immer dkim=pass/spf=pass")
	require.Empty(t, page.NextCursor)
}

func TestDomainStatsRepository_Query_SeparatesDistinctDomains(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewDomainStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "a.example", reportID: "sep-a", begin: begin})
	saveTestReport(t, reports, reportOpts{domain: "b.example", reportID: "sep-b", begin: begin})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	page, err := stats.Query(ctx, domainstats.Query{Period: &period, SortField: domainstats.SortByDomain})
	require.NoError(t, err)
	require.Len(t, page.Stats, 2)
	require.Equal(t, "a.example", page.Stats[0].Domain.String())
	require.Equal(t, "b.example", page.Stats[1].Domain.String())
}

func TestDomainStatsRepository_Query_SortByVolume_PaginatesWithCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewDomainStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "big.example", reportID: "vol-big"})
	saveTestReport(t, reports, reportOpts{domain: "mid.example", reportID: "vol-mid"})
	saveTestReport(t, reports, reportOpts{domain: "small.example", reportID: "vol-small"})
	// newTestReport erzeugt für jeden Report genau ein Record mit
	// message_count 3 — für unterschiedliches Volumen je Domain zusätzliche
	// Reports auf dieselbe Domain speichern.
	saveTestReport(t, reports, reportOpts{domain: "big.example", reportID: "vol-big-2"})
	saveTestReport(t, reports, reportOpts{domain: "big.example", reportID: "vol-big-3"})
	saveTestReport(t, reports, reportOpts{domain: "mid.example", reportID: "vol-mid-2"})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(48*time.Hour))
	require.NoError(t, err)

	first, err := stats.Query(ctx, domainstats.Query{Period: &period, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Stats, 2)
	require.Equal(t, "big.example", first.Stats[0].Domain.String())
	require.Equal(t, 9, first.Stats[0].TotalCount)
	require.Equal(t, "mid.example", first.Stats[1].Domain.String())
	require.NotEmpty(t, first.NextCursor)

	second, err := stats.Query(ctx, domainstats.Query{Period: &period, Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Stats, 1)
	require.Equal(t, "small.example", second.Stats[0].Domain.String())
	require.Empty(t, second.NextCursor)
}

func TestDomainStatsRepository_Query_FiltersByDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewDomainStatsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveTestReport(t, reports, reportOpts{domain: "match.example", reportID: "filter-match", begin: begin})
	saveTestReport(t, reports, reportOpts{domain: "other.example", reportID: "filter-other", begin: begin})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	match, err := stats.Query(ctx, domainstats.Query{Period: &period, Domain: "match.example"})
	require.NoError(t, err)
	require.Len(t, match.Stats, 1)
	require.Equal(t, "match.example", match.Stats[0].Domain.String())
}
