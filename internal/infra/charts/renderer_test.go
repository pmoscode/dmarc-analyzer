package charts_test

import (
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/charts"
)

func sourceIP(t *testing.T, s string) report.SourceIP {
	t.Helper()
	ip, err := report.NewSourceIP(s)
	require.NoError(t, err)
	return ip
}

func TestRenderer_DailyVolumeChart_ProducesNonEmptyImage(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.DailyVolumeChart([]analysis.DailyVolume{
		{Day: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Pass: 10, Fail: 2},
		{Day: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Pass: 5, Fail: 5},
	})
	require.NoError(t, err)
	requireNonTrivialImage(t, img)
}

func TestRenderer_DailyVolumeChart_EmptyData_ReturnsBlankImageNoError(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.DailyVolumeChart(nil)
	require.NoError(t, err)
	require.NotNil(t, img)
}

func TestRenderer_TopSourcesChart_ProducesNonEmptyImage(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.TopSourcesChart([]analysis.SourceVolume{
		{SourceIP: sourceIP(t, "203.0.113.1"), Total: 50, PassRate: 0.9},
		{SourceIP: sourceIP(t, "203.0.113.2"), Total: 20, PassRate: 0.1},
	})
	require.NoError(t, err)
	requireNonTrivialImage(t, img)
}

func TestRenderer_DispositionChart_ProducesNonEmptyImage(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.DispositionChart(map[report.Disposition]int{
		report.DispositionNone:       80,
		report.DispositionQuarantine: 15,
		report.DispositionReject:     5,
	})
	require.NoError(t, err)
	requireNonTrivialImage(t, img)
}

func TestRenderer_DispositionChart_EmptyData_ReturnsBlankImageNoError(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.DispositionChart(map[report.Disposition]int{})
	require.NoError(t, err)
	require.NotNil(t, img)
}

func TestRenderer_HeatmapChart_ProducesImageSizedByData(t *testing.T) {
	r := charts.NewRenderer()

	data := analysis.Heatmap{
		Sources: []report.SourceIP{sourceIP(t, "203.0.113.1"), sourceIP(t, "203.0.113.2")},
		Days: []time.Time{
			time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
		},
		Cells: [][]analysis.HeatmapCell{
			{{PassRate: 1, HasData: true}, {HasData: false}, {PassRate: 0.5, HasData: true}},
			{{PassRate: 0, HasData: true}, {PassRate: 0.2, HasData: true}, {HasData: false}},
		},
	}

	img, err := r.HeatmapChart(data)
	require.NoError(t, err)
	requireNonTrivialImage(t, img)

	// Größer als eine leere Standardgröße, da 2 Quellen × 3 Tage plus
	// Beschriftungsrand gezeichnet werden.
	require.Greater(t, img.Bounds().Dx(), 90)
	require.Greater(t, img.Bounds().Dy(), 24)
}

func TestRenderer_HeatmapChart_NoSources_ReturnsBlankImageNoError(t *testing.T) {
	r := charts.NewRenderer()

	img, err := r.HeatmapChart(analysis.Heatmap{})
	require.NoError(t, err)
	require.NotNil(t, img)
}

// requireNonTrivialImage prüft, dass tatsächlich mehr als eine einzelne
// Farbe gezeichnet wurde — ein Regressionstest gegen "Render lief durch,
// hat aber nur ein leeres/einfarbiges Bild erzeugt" (z. B. falsch
// verdrahtete Farben oder eine vergessene Fill-Operation).
func requireNonTrivialImage(t *testing.T, img image.Image) {
	t.Helper()
	require.NotNil(t, img)

	bounds := img.Bounds()
	require.Greater(t, bounds.Dx(), 0)
	require.Greater(t, bounds.Dy(), 0)

	first := img.At(bounds.Min.X, bounds.Min.Y)
	distinctColors := false
	for y := bounds.Min.Y; y < bounds.Max.Y && !distinctColors; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.At(x, y) != first {
				distinctColors = true
				break
			}
		}
	}
	require.True(t, distinctColors, "bild besteht nur aus einer einzigen farbe — vermutlich nichts gezeichnet")
}
