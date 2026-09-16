package statistics_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

type fakeDashboardRepository struct {
	stats        analysis.Statistics
	dailyVolumes []analysis.DailyVolume
	topSources   []analysis.SourceVolume
	heatmap      analysis.Heatmap

	dailyVolumesErr error
	topSourcesErr   error
	heatmapErr      error

	topSourcesLimit int
	heatmapLimit    int
}

func (f *fakeDashboardRepository) Compute(context.Context, analysis.Query) (analysis.Statistics, error) {
	return f.stats, nil
}

func (f *fakeDashboardRepository) DailyVolumes(context.Context, analysis.Query) ([]analysis.DailyVolume, error) {
	if f.dailyVolumesErr != nil {
		return nil, f.dailyVolumesErr
	}
	return f.dailyVolumes, nil
}

func (f *fakeDashboardRepository) TopSources(_ context.Context, _ analysis.Query, limit int) ([]analysis.SourceVolume, error) {
	f.topSourcesLimit = limit
	if f.topSourcesErr != nil {
		return nil, f.topSourcesErr
	}
	return f.topSources, nil
}

func (f *fakeDashboardRepository) Heatmap(_ context.Context, _ analysis.Query, limit int) (analysis.Heatmap, error) {
	f.heatmapLimit = limit
	if f.heatmapErr != nil {
		return analysis.Heatmap{}, f.heatmapErr
	}
	return f.heatmap, nil
}

func TestUseCase_Dashboard_BundlesAllData(t *testing.T) {
	t.Parallel()

	ip, err := report.NewSourceIP("203.0.113.1")
	require.NoError(t, err)

	repo := &fakeDashboardRepository{
		stats:        analysis.Statistics{TotalMessages: 42},
		dailyVolumes: []analysis.DailyVolume{{Day: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Pass: 10}},
		topSources:   []analysis.SourceVolume{{SourceIP: ip, Total: 10}},
		heatmap:      analysis.Heatmap{Sources: []report.SourceIP{ip}},
	}
	uc := &statistics.UseCase{Repository: repo}

	got, err := uc.Dashboard(context.Background(), analysis.Query{
		Period: period(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)),
	})
	require.NoError(t, err)

	require.Equal(t, 42, got.Comparison.Current.TotalMessages)
	require.Equal(t, repo.dailyVolumes, got.DailyVolumes)
	require.Equal(t, repo.topSources, got.TopSources)
	require.Equal(t, repo.heatmap, got.Heatmap)
	require.Equal(t, 10, repo.topSourcesLimit)
	require.Equal(t, 10, repo.heatmapLimit)
}

func TestUseCase_Dashboard_ForwardsErrorsFromEachSource(t *testing.T) {
	t.Parallel()
	q := analysis.Query{Period: period(t, time.Now(), time.Now().Add(time.Hour))}

	t.Run("DailyVolumes", func(t *testing.T) {
		t.Parallel()
		uc := &statistics.UseCase{Repository: &fakeDashboardRepository{dailyVolumesErr: errTest}}
		_, err := uc.Dashboard(context.Background(), q)
		require.ErrorIs(t, err, errTest)
	})

	t.Run("TopSources", func(t *testing.T) {
		t.Parallel()
		uc := &statistics.UseCase{Repository: &fakeDashboardRepository{topSourcesErr: errTest}}
		_, err := uc.Dashboard(context.Background(), q)
		require.ErrorIs(t, err, errTest)
	})

	t.Run("Heatmap", func(t *testing.T) {
		t.Parallel()
		uc := &statistics.UseCase{Repository: &fakeDashboardRepository{heatmapErr: errTest}}
		_, err := uc.Dashboard(context.Background(), q)
		require.ErrorIs(t, err, errTest)
	})
}

// fakeEnricher liefert für jede IP ein festes, konfigurierbares
// Enrichment — keine echte DNS-Auflösung nötig, um die Label-Zuordnung
// in Dashboard() zu prüfen.
type fakeEnricher struct {
	byIP map[string]sources.Enrichment
}

func (f *fakeEnricher) Enrich(_ context.Context, ip report.SourceIP) sources.Enrichment {
	return f.byIP[ip.String()]
}

func TestUseCase_Dashboard_EnrichesTopSourcesAndHeatmapLabels(t *testing.T) {
	t.Parallel()

	ipKnown, err := report.NewSourceIP("203.0.113.1")
	require.NoError(t, err)
	ipUnknown, err := report.NewSourceIP("203.0.113.2")
	require.NoError(t, err)

	repo := &fakeDashboardRepository{
		stats:      analysis.Statistics{TotalMessages: 42},
		topSources: []analysis.SourceVolume{{SourceIP: ipKnown, Total: 10}, {SourceIP: ipUnknown, Total: 5}},
		heatmap:    analysis.Heatmap{Sources: []report.SourceIP{ipKnown, ipUnknown}},
	}
	uc := &statistics.UseCase{
		Repository: repo,
		Enricher: &fakeEnricher{byIP: map[string]sources.Enrichment{
			ipKnown.String(): {Hostname: "mail.example.com", Service: "Google Workspace"},
		}},
	}

	got, err := uc.Dashboard(context.Background(), analysis.Query{
		Period: period(t, time.Now(), time.Now().Add(time.Hour)),
	})
	require.NoError(t, err)

	require.Equal(t, "Google Workspace", got.TopSources[0].Label,
		"erkannter Dienst geht dem reinen PTR-Hostnamen vor")
	require.Empty(t, got.TopSources[1].Label, "ohne Enrichment bleibt Label leer — Renderer fällt auf die IP zurück")

	require.Equal(t, []string{"Google Workspace", ""}, got.Heatmap.SourceLabels,
		"Heatmap-Zeilen übernehmen dieselbe Anreicherung wie die Top-Sendequellen")
}

func TestUseCase_Dashboard_NilEnricher_LeavesLabelsEmpty(t *testing.T) {
	t.Parallel()

	ip, err := report.NewSourceIP("203.0.113.1")
	require.NoError(t, err)

	repo := &fakeDashboardRepository{
		topSources: []analysis.SourceVolume{{SourceIP: ip, Total: 10}},
		heatmap:    analysis.Heatmap{Sources: []report.SourceIP{ip}},
	}
	uc := &statistics.UseCase{Repository: repo}

	got, err := uc.Dashboard(context.Background(), analysis.Query{
		Period: period(t, time.Now(), time.Now().Add(time.Hour)),
	})
	require.NoError(t, err)

	require.Empty(t, got.TopSources[0].Label)
	require.Nil(t, got.Heatmap.SourceLabels)
}
