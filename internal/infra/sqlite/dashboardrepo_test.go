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

func TestStatisticsRepository_DailyVolumes_BucketsByReportDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	day1 := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 3, 3, 0, 0, 0, time.UTC)

	saveReportWithRecords(t, reports, "dv-day1", day1, []recordSpec{
		{count: 10, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
		{count: 4, disposition: report.DispositionReject, dkim: report.AuthResultFail, spf: report.AuthResultFail},
	})
	saveReportWithRecords(t, reports, "dv-day2", day2, []recordSpec{
		{count: 7, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass},
	})

	period, err := report.NewDateRange(day1.Add(-time.Hour), day2.Add(24*time.Hour))
	require.NoError(t, err)

	got, err := stats.DailyVolumes(ctx, analysis.Query{Period: period})
	require.NoError(t, err)
	require.Len(t, got, 2, "zwei unterschiedliche Report-Tage")

	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), got[0].Day)
	require.Equal(t, 10, got[0].Pass)
	require.Equal(t, 4, got[0].Fail)

	require.Equal(t, time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), got[1].Day)
	require.Equal(t, 7, got[1].Pass)
	require.Equal(t, 0, got[1].Fail)
}

func TestStatisticsRepository_TopSources_SortedByVolumeDescending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	saveReportWithRecords(t, reports, "top-sources", begin, []recordSpec{
		{count: 5, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.1"},
		{count: 50, disposition: report.DispositionNone, dkim: report.AuthResultFail, spf: report.AuthResultFail, sourceIP: "203.0.113.2"},
		{count: 20, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultFail, sourceIP: "203.0.113.3"},
	})

	period, err := report.NewDateRange(begin.Add(-time.Hour), begin.Add(2*time.Hour))
	require.NoError(t, err)

	got, err := stats.TopSources(ctx, analysis.Query{Period: period}, 2)
	require.NoError(t, err)
	require.Len(t, got, 2, "limit muss respektiert werden")

	require.Equal(t, "203.0.113.2", got[0].SourceIP.String())
	require.Equal(t, 50, got[0].Total)
	require.InDelta(t, 0.0, got[0].PassRate, 0.0001)

	require.Equal(t, "203.0.113.3", got[1].SourceIP.String())
	require.Equal(t, 20, got[1].Total)
	require.InDelta(t, 1.0, got[1].PassRate, 0.0001)
}

func TestStatisticsRepository_Heatmap_MarksMissingDaysAsNoData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := newTestDB(t)
	reports := sqlite.NewReportRepository(db)
	stats := sqlite.NewStatisticsRepository(db)

	day1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)

	saveReportWithRecords(t, reports, "heatmap-day1", day1, []recordSpec{
		{count: 10, disposition: report.DispositionNone, dkim: report.AuthResultPass, spf: report.AuthResultPass, sourceIP: "203.0.113.1"},
	})
	saveReportWithRecords(t, reports, "heatmap-day3", day3, []recordSpec{
		{count: 4, disposition: report.DispositionReject, dkim: report.AuthResultFail, spf: report.AuthResultFail, sourceIP: "203.0.113.1"},
	})

	period, err := report.NewDateRange(day1, day3.Add(24*time.Hour))
	require.NoError(t, err)

	got, err := stats.Heatmap(ctx, analysis.Query{Period: period}, 5)
	require.NoError(t, err)

	require.Equal(t, []string{"203.0.113.1"}, ipStrings(got.Sources))
	require.Len(t, got.Days, 3, "1., 2. und 3. September")

	require.True(t, got.Cells[0][0].HasData)
	require.InDelta(t, 1.0, got.Cells[0][0].PassRate, 0.0001)
	require.Equal(t, 10, got.Cells[0][0].Total)

	require.False(t, got.Cells[0][1].HasData, "2. September hat keine Nachrichten dieser Quelle")
	require.Zero(t, got.Cells[0][1].Total)

	require.True(t, got.Cells[0][2].HasData)
	require.InDelta(t, 0.0, got.Cells[0][2].PassRate, 0.0001)
	require.Equal(t, 4, got.Cells[0][2].Total)
}

func TestStatisticsRepository_Heatmap_NoSources_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stats := sqlite.NewStatisticsRepository(newTestDB(t))

	period, err := report.NewDateRange(time.Now(), time.Now().Add(time.Hour))
	require.NoError(t, err)

	got, err := stats.Heatmap(ctx, analysis.Query{Period: period}, 5)
	require.NoError(t, err)
	require.Empty(t, got.Sources)
	require.Empty(t, got.Days)
}

func ipStrings(ips []report.SourceIP) []string {
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}
