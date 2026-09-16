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
	"image/draw"
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

	// donutWidth gibt der Disposition-Donut zusätzlichen horizontalen
	// Rand für Slice-Beschriftungen (siehe DispositionChart).
	donutWidth = 900
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
		return blankImage(defaultWidth, defaultHeight), nil
	}

	bars := make([]chart.StackedBar, len(data))
	for i, d := range data {
		bars[i] = chart.StackedBar{
			Name: d.Day.Format("02.01."),
			Values: []chart.Value{
				{Label: "Bestanden", Value: float64(d.Pass), Style: barStyle(colorPass)},
				{Label: "Fehlgeschlagen", Value: float64(d.Fail), Style: barStyle(colorFail)},
			},
		}
	}

	return renderPNG(chart.StackedBarChart{
		Width: defaultWidth, Height: defaultHeight,
		Bars: bars,
		// TextWrapNone: siehe TopSourcesChart — dieselbe Falle betrifft
		// auch Datumsbeschriftungen ("02.01."), sobald bei einem großen
		// Zeitraum viele schmale Balken nebeneinander stehen.
		XAxis: chart.Style{TextWrap: chart.TextWrapNone},
		// Etwas mehr Abstand am unteren Rand (Default wäre 50px) — die
		// Datumsbeschriftung sitzt sonst nur wenige Pixel über der
		// unteren Bildkante.
		Background: chart.Style{Padding: chart.Box{Bottom: 70}},
	})
}

// TopSourcesChart zeichnet die Sendequellen als Balkendiagramm, je Balken
// eingefärbt nach Pass-Rate (rot = 0 %, grün = 100 %).
func (Renderer) TopSourcesChart(data []analysis.SourceVolume) (image.Image, error) {
	if len(data) == 0 {
		return blankImage(defaultWidth, defaultHeight), nil
	}

	bars := make([]chart.Value, len(data))
	for i, s := range data {
		bars[i] = chart.Value{
			Label: sourceVolumeLabel(s),
			Value: float64(s.Total),
			Style: barStyle(passRateColor(s.PassRate)),
		}
	}

	return renderPNG(chart.BarChart{
		Width: defaultWidth, Height: defaultHeight,
		Bars: bars,
		// go-chart/v2 bricht lange, leerzeichenfreie Achsenbeschriftungen
		// (wie IP-Adressen) mit dem Default-Stil (TextWrapWord) fehlerhaft
		// um: WrapFitWord() hängt bei Strings ohne Leerzeichen eine leere
		// erste Zeile an, wodurch der eigentliche Text bei schmalen Balken
		// unterhalb des sichtbaren Diagramms landet — er wird berechnet,
		// aber nie gezeichnet sichtbar (siehe text.go WrapFitWord). Mit
		// vielen Sendequellen (schmale Balken) und langen IPs trat das
		// zuverlässig auf. TextWrapNone verhindert das Umbrechen, die
		// 90°-Drehung lässt die Beschriftung trotzdem in die schmale
		// Balkenspalte passen, ohne Nachbarbalken zu überlappen.
		XAxis: chart.Style{TextWrap: chart.TextWrapNone, TextRotationDegrees: 90},
		// Mehr Platz am unteren Rand für die jetzt vertikal stehenden,
		// bis zu ~15 Zeichen langen IP-Beschriftungen (Default wäre 50px).
		Background: chart.Style{Padding: chart.Box{Bottom: 130}},
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
			Style: barStyle(dispositionColor(d)),
		})
	}
	if len(values) == 0 {
		return blankImage(donutWidth, defaultHeight), nil
	}
	if len(values) == 1 {
		// go-chart/v2's DonutChart.drawSlices hat für genau einen Wert
		// einen eigenen Code-Pfad, der den Kreis zwar aufspannt, aber nie
		// füllt oder zeichnet (Circle() baut nur den Pfad auf, ohne
		// Fill()/FillStroke() — siehe raster_renderer.go: "does not apply
		// the fill or stroke"). Ergebnis wäre ein komplett leeres, weißes
		// Bild. Das ist bei uns der Normalfall, sobald alle Nachrichten
		// dieselbe Disposition haben (z. B. 100 % "Keine Maßnahme"). Als
		// Workaround splitten wir den einen Wert in zwei gleich gefärbte
		// Hälften auf — das nimmt den (korrekt implementierten)
		// Mehrwerte-Pfad und ergibt einen vollständig gefüllten Kreis;
		// da FillColor == StrokeColor (siehe barStyle) ist die Nahtstelle
		// zwischen den beiden Hälften unsichtbar.
		half := values[0]
		half.Value /= 2
		second := half
		second.Label = "" // sonst erscheint das Label zweimal auf dem Kreis
		values = []chart.Value{half, second}
	}

	return renderPNG(chart.DonutChart{
		// Breiter als hoch: der Kreisdurchmesser richtet sich nach dem
		// kleineren der beiden Werte (Height), die zusätzliche Breite
		// bleibt als Rand für die Slice-Beschriftungen (z. B.
		// "Zurückgewiesen"), die sonst am Bildrand abgeschnitten würden.
		Width: donutWidth, Height: defaultWidth,
		Values: values,
	})
}

// sourceVolumeLabel zeigt den von app/statistics angereicherten Namen
// (erkannter Dienst oder PTR-Hostname), fällt ohne Anreicherung auf die
// reine IP-Adresse zurück.
func sourceVolumeLabel(s analysis.SourceVolume) string {
	if s.Label != "" {
		return s.Label
	}
	return s.SourceIP.String()
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

// barStyle setzt neben FillColor auch StrokeColor auf dieselbe Farbe.
// go-chart zeichnet um jeden Balken/jede Slice einen 3-4px breiten Rahmen
// und füllt dessen Farbe standardmäßig aus einer fixen, durchrotierenden
// Palette (GetSeriesColor(index)) statt aus unserer FillColor — bei sehr
// kleinen Werten (Balkenhöhe kleiner als der Rahmen) verdeckt dieser
// Standard-Rahmen die eigentliche Füllfarbe fast vollständig, sodass
// Balken mit geringem Volumen in zufällig wirkenden Palettenfarben statt
// in der beabsichtigten Pass-Rate-Farbe erscheinen. Ohne diesen Fix wäre
// das insbesondere bei den Top-Sendequellen sichtbar, wenn eine Quelle
// die übrigen Quellen im Volumen stark überragt.
func barStyle(c drawing.Color) chart.Style {
	return chart.Style{FillColor: c, StrokeColor: c}
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
	return flattenOnWhite(img), nil
}

// flattenOnWhite kompositiert img auf einen opaken weißen Hintergrund.
// go-chart/v2 füllt den äußeren Rand außerhalb der eigentlichen
// Zeichenfläche nicht bei jedem Diagrammtyp (StackedBarChart ruft anders
// als BarChart/DonutChart kein drawBackground() auf) — Achsen-/
// Balkenbeschriftungen in diesem Rand landen dann auf transparentem
// Grund. In einer normalen (hellen) Bildvorschau fällt das nicht auf,
// im dunklen Anwendungs-Theme scheint dort aber der dunkle
// Fensterhintergrund durch, wodurch Beschriftungen wie ausgeblichen auf
// Schwarz statt auf Weiß wirken. Ohne diesen Flatten-Schritt wäre jedes
// Diagramm von diesem go-chart-Verhalten abhängig statt es einheitlich
// selbst zu garantieren.
func flattenOnWhite(img image.Image) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(b)
	draw.Draw(out, b, image.NewUniform(chart.ColorWhite), image.Point{}, draw.Src)
	draw.Draw(out, b, img, b.Min, draw.Over)
	return out
}

// blankImage liefert ein leeres, weißes Bild für den Fall ohne Daten —
// die aufrufende UI (internal/ui/dashboard) zeigt in diesem Fall ohnehin
// einen Leerzustand statt des Diagramms; ein Fehler wäre hier unnötig
// streng, ein leeres Bild ein harmloser, sicherer Rückgabewert.
func blankImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := image.NewUniform(chart.ColorWhite)
	drawFill(img, white)
	return img
}
