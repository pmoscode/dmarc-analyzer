package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appstatistics "github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
)

type fakeStatsRepository struct {
	stats analysis.Statistics
}

func (f *fakeStatsRepository) Compute(context.Context, analysis.Query) (analysis.Statistics, error) {
	return f.stats, nil
}

// DailyVolumes/TopSources/Heatmap: runStats (cmd_stats.go) ruft nur
// Compute auf — leere Stubs, nur damit fakeStatsRepository
// analysis.Repository weiterhin vollständig erfüllt.
func (f *fakeStatsRepository) DailyVolumes(context.Context, analysis.Query) ([]analysis.DailyVolume, error) {
	return nil, nil
}

func (f *fakeStatsRepository) TopSources(context.Context, analysis.Query, int) ([]analysis.SourceVolume, error) {
	return nil, nil
}

func (f *fakeStatsRepository) Heatmap(context.Context, analysis.Query, int) (analysis.Heatmap, error) {
	return analysis.Heatmap{}, nil
}

func TestRunStats_InvalidDays_ReturnsError(t *testing.T) {
	t.Parallel()

	a := &app{stats: &appstatistics.UseCase{Repository: &fakeStatsRepository{}}}
	err := runStats(context.Background(), a, []string{"--days", "0"})
	require.Error(t, err)
}

func TestRunStats_Success(t *testing.T) {
	t.Parallel()

	a := &app{stats: &appstatistics.UseCase{Repository: &fakeStatsRepository{
		stats: analysis.Statistics{TotalMessages: 5, PassRate: 1},
	}}}
	err := runStats(context.Background(), a, []string{"--days", "7"})
	require.NoError(t, err)
}
