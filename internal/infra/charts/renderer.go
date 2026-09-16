// Package charts implementiert den Port analysis.ChartRenderer gegen
// github.com/wcharczuk/go-chart/v2 und liefert image.Image zur Einbettung
// in Fyne-Widgets (IMPLEMENTIERUNG.md Abschnitt 8.2, AP 6).
//
// Beschriftungen in den Diagrammen sind hier als literale, deutsche
// Zeichenketten gehalten statt aus internal/ui/i18n zu kommen — i18n ist
// eine Abhängigkeit der UI-Schicht, die infra nicht importieren darf
// (Abhängigkeitsrichtung, siehe AGENTS.md). Die kleine Dopplung ist
// bewusst in Kauf genommen.
package charts

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"

	chart "github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// defaultWidth/-Height gelten für alle Diagramme außer der Heatmap, deren
// Größe von der Anzahl Quellen/Tage abhängt.
const (
	defaultWidth  = 640
	defaultHeight = 360
)

var (
	colorPass    = drawing.Color{R: 0x2e, G: 0xa0, B: 0x4f, A: 0xff}
	colorFail    = drawing.Color{R: 0xd6, G: 0x3b, B: 0x3b, A: 0xff}
	colorUnknown = drawing.Color{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff}
)

// Renderer implementiert analysis.ChartRenderer.
type Renderer struct{}

var _ analysis.ChartRenderer = Renderer{}

// NewRenderer erzeugt einen einsatzbereiten Renderer — zustandslos, kann
// als Wert verwendet werden.
func NewRenderer() Renderer { return Renderer{} }

// DailyVolumeChart zeichnet die Zeitreihe als gestapeltes Balkendiagramm
// (ein Balken je Tag, Pass/Fail gestapelt).
func (Renderer) DailyVolumeChart(data []analysis.DailyVolume) (image.Image, error) {
	if len(data) == 0 {
		return blankImage(defaultHeight), nil
	}

	bars := make([]chart.StackedBar, len(data))
	for i, d := range data {
		bars[i] = chart.StackedBar{
			Name: d.Day.Format("02.01."),
			Values: []chart.Value{
				{Label: "Bestanden", Value: float64(d.Pass), Style: chart.Style{FillColor: colorPass}},
				{Label: "Fehlgeschlagen", Value: float64(d.Fail), Style: chart.Style{FillColor: colorFail}},
			},
		}
	}

	return renderPNG(chart.StackedBarChart{
		Width: defaultWidth, Height: defaultHeight,
		Bars: bars,
	})
}

// TopSourcesChart zeichnet die Sendequellen als Balkendiagramm, je Balken
// eingefärbt nach Pass-Rate (rot = 0 %, grün = 100 %).
func (Renderer) TopSourcesChart(data []analysis.SourceVolume) (image.Image, error) {
	if len(data) == 0 {
		return blankImage(defaultHeight), nil
	}

	bars := make([]chart.Value, len(data))
	for i, s := range data {
		bars[i] = chart.Value{
			Label: s.SourceIP.String(),
			Value: float64(s.Total),
			Style: chart.Style{FillColor: passRateColor(s.PassRate)},
		}
	}

	return renderPNG(chart.BarChart{
		Width: defaultWidth, Height: defaultHeight,
		Bars: bars,
	})
}

// dispositionOrder legt eine feste, deterministische Reihenfolge für den
// Donut fest — die Iteration über eine map wäre nicht reproduzierbar.
var dispositionOrder = []report.Disposition{
	report.DispositionNone, report.DispositionQuarantine, report.DispositionReject, report.DispositionUnknown,
}

// DispositionChart zeichnet die Verteilung der Dispositions als Donut.
func (Renderer) DispositionChart(data map[report.Disposition]int) (image.Image, error) {
	var values []chart.Value
	for _, d := range dispositionOrder {
		count := data[d]
		if count <= 0 {
			continue
		}
		values = append(values, chart.Value{
			Label: dispositionLabel(d),
			Value: float64(count),
			Style: chart.Style{FillColor: dispositionColor(d)},
		})
	}
	if len(values) == 0 {
		return blankImage(defaultWidth), nil
	}

	return renderPNG(chart.DonutChart{
		Width: defaultWidth, Height: defaultWidth,
		Values: values,
	})
}

func dispositionLabel(d report.Disposition) string {
	switch d {
	case report.DispositionNone:
		return "Keine Maßnahme"
	case report.DispositionQuarantine:
		return "Quarantäne"
	case report.DispositionReject:
		return "Zurückgewiesen"
	default:
		return "Unbekannt"
	}
}

func dispositionColor(d report.Disposition) drawing.Color {
	switch d {
	case report.DispositionNone:
		return colorPass
	case report.DispositionQuarantine:
		return drawing.Color{R: 0xe0, G: 0x9a, B: 0x1c, A: 0xff}
	case report.DispositionReject:
		return colorFail
	default:
		return colorUnknown
	}
}

// passRateColor interpoliert linear zwischen Rot (0 %) und Grün (100 %).
func passRateColor(rate float64) drawing.Color {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	lerp := func(a, b byte) byte { return byte(float64(a) + rate*(float64(b)-float64(a))) }
	return drawing.Color{
		R: lerp(colorFail.R, colorPass.R),
		G: lerp(colorFail.G, colorPass.G),
		B: lerp(colorFail.B, colorPass.B),
		A: 0xff,
	}
}

// pngRenderable ist die von BarChart, StackedBarChart und DonutChart
// gemeinsam erfüllte Schnittstelle — genug, um sie über eine gemeinsame
// Funktion nach PNG zu rendern.
type pngRenderable interface {
	Render(rp chart.RendererProvider, w io.Writer) error
}

func renderPNG(c pngRenderable) (image.Image, error) {
	var buf bytes.Buffer
	if err := c.Render(chart.PNG, &buf); err != nil {
		return nil, fmt.Errorf("diagramm konnte nicht gezeichnet werden: %w", err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		return nil, fmt.Errorf("gezeichnetes diagramm konnte nicht dekodiert werden: %w", err)
	}
	return img, nil
}

// blankImage liefert ein leeres, weißes Bild für den Fall ohne Daten —
// die aufrufende UI (internal/ui/dashboard) zeigt in diesem Fall ohnehin
// einen Leerzustand statt des Diagramms; ein Fehler wäre hier unnötig
// streng, ein leeres Bild ein harmloser, sicherer Rückgabewert.
func blankImage(height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, defaultWidth, height))
	white := image.NewUniform(chart.ColorWhite)
	drawFill(img, white)
	return img
}
