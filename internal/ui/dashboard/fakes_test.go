package dashboard

import (
	"context"
	"image"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errTest = fakeErr("testfehler")

// fakeRepository implementiert analysis.Repository mit fest verdrahteten
// Rückgabewerten.
type fakeRepository struct {
	stats        analysis.Statistics
	dailyVolumes []analysis.DailyVolume
	topSources   []analysis.SourceVolume
	heatmap      analysis.Heatmap
	computeErr   error
	lastQuery    analysis.Query
}

func (f *fakeRepository) Compute(_ context.Context, q analysis.Query) (analysis.Statistics, error) {
	f.lastQuery = q
	if f.computeErr != nil {
		return analysis.Statistics{}, f.computeErr
	}
	return f.stats, nil
}

func (f *fakeRepository) DailyVolumes(_ context.Context, q analysis.Query) ([]analysis.DailyVolume, error) {
	// Anders als Compute (von ComputeWithTrend zweimal mit
	// unterschiedlichem Zeitraum aufgerufen — aktuelle und Vorperiode)
	// wird DailyVolumes von Dashboard() genau einmal mit dem
	// ursprünglichen, ungeschobenen Zeitraum aufgerufen — deshalb hier
	// und nicht in Compute erfasst.
	f.lastQuery = q
	return f.dailyVolumes, nil
}

func (f *fakeRepository) TopSources(context.Context, analysis.Query, int) ([]analysis.SourceVolume, error) {
	return f.topSources, nil
}

func (f *fakeRepository) Heatmap(context.Context, analysis.Query, int) (analysis.Heatmap, error) {
	return f.heatmap, nil
}

// fakeChartRenderer implementiert analysis.ChartRenderer ohne go-chart —
// liefert je Aufruf ein winziges, festes Bild, damit die Tests nicht von
// der echten Zeichenlogik abhängen (die hat renderer_test.go bereits).
type fakeChartRenderer struct {
	err error
}

func (f *fakeChartRenderer) tinyImage() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 2, 2))
}

func (f *fakeChartRenderer) DailyVolumeChart([]analysis.DailyVolume) (image.Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tinyImage(), nil
}

func (f *fakeChartRenderer) TopSourcesChart([]analysis.SourceVolume) (image.Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tinyImage(), nil
}

func (f *fakeChartRenderer) DispositionChart(map[report.Disposition]int) (image.Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tinyImage(), nil
}

func (f *fakeChartRenderer) HeatmapChart(analysis.Heatmap) (image.Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tinyImage(), nil
}
