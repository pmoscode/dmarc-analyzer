// Package dashboard zeigt Kennzahlen-Kacheln, Zeitreihe, Top-Absender
// und die Verteilung der Dispositions (IMPLEMENTIERUNG.md Abschnitt 10.2
// und 10.3).
package dashboard

import (
	"context"
	"fmt"
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/glossary"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// View zeigt die Übersicht: Kennzahlen-Kacheln inklusive Trend zur
// Vorperiode sowie vier Diagramme. runBackground/fyne.Do():
// siehe internal/ui/settings.View (AGENTS.md).
type View struct {
	widget.BaseWidget

	stats         *statistics.UseCase
	charts        analysis.ChartRenderer
	window        fyne.Window
	runBackground func(f func())

	filterPeriod report.DateRange
	filterDomain string

	tileTotal, tilePass, tileDKIM, tileSPF, tileSources, tileTrendLabel *widget.Label
	// tileTrendIcon: der Trendpfeil ist ein Fyne-Icon (Vektorgrafik) statt
	// eines Unicode-Pfeilzeichens ("→"/"▲"/"▼") im Label-Text — die von
	// Fyne gebündelte Standardschrift deckt diese Symbole nicht ab und
	// zeigt stattdessen ein Ersatzzeichen (Tofu-Box, "�").
	tileTrendIcon *widget.Icon

	dailyPanel *chartPanel
	topPanel   *chartPanel
	dispPanel  *chartPanel
	heatPanel  *chartPanel

	content   *fyne.Container
	scroll    *container.Scroll
	container *fyne.Container
}

// NewView erzeugt die Übersicht mit leerem Zustand. SetFilter() lädt die
// erste Seite — bewusst nicht automatisch im Konstruktor, siehe
// settings.NewView für dieselbe Begründung.
func NewView(stats *statistics.UseCase, charts analysis.ChartRenderer, window fyne.Window) *View {
	v := &View{
		stats:         stats,
		charts:        charts,
		window:        window,
		runBackground: func(f func()) { go f() },
	}

	v.tileTotal = widget.NewLabel("")
	v.tilePass = widget.NewLabel("")
	v.tileDKIM = widget.NewLabel("")
	v.tileSPF = widget.NewLabel("")
	v.tileSources = widget.NewLabel("")
	v.tileTrendLabel = widget.NewLabel("")
	v.tileTrendIcon = widget.NewIcon(nil)

	tiles := container.NewGridWithColumns(3,
		newTile(i18n.DashboardTileTotalMessages, "", v.tileTotal, window),
		newTile(i18n.DashboardTilePassRate, "DMARC-Pass-Rate", v.tilePass, window),
		newTile(i18n.DashboardTileDKIMAlignment, "Alignment", v.tileDKIM, window),
		newTile(i18n.DashboardTileSPFAlignment, "Alignment", v.tileSPF, window),
		newTile(i18n.DashboardTileDistinctSources, "Quell-IP", v.tileSources, window),
	)

	v.dailyPanel = newChartPanel(i18n.DashboardChartDailyVolume, v.exportChart(func() image.Image { return v.dailyPanel.image() }), window)
	v.topPanel = newChartPanel(i18n.DashboardChartTopSources, v.exportChart(func() image.Image { return v.topPanel.image() }), window)
	v.dispPanel = newChartPanel(i18n.DashboardChartDisposition, v.exportChart(func() image.Image { return v.dispPanel.image() }), window)
	v.heatPanel = newChartPanel(i18n.DashboardChartHeatmap, v.exportChart(func() image.Image { return v.heatPanel.image() }), window)

	// Panels von Anfang an mit einem leeren, aber KORREKT GROSSEN Platzhalter
	// füllen (statt img.Image nil zu lassen): so bekommt jedes Panel schon
	// beim allerersten, echten Resize-Durchlauf (Fenster wird erzeugt/
	// gezeigt) seine endgültige Höhe zugewiesen. Würde stattdessen erst
	// später — nachdem die echten Daten asynchron geladen sind — auf die
	// richtige Größe hochgewachsen (über setImage()/Refresh()), hängt das
	// von einer Refresh-Kaskade durch mehrere verschachtelte Container
	// (Scroll → VBox → Panel) ab, die im echten (GPU-beschleunigten)
	// Fenster beobachtet unzuverlässig war (der untere Rand blieb
	// abgeschnitten, obwohl dieselbe Kaskade in Tests korrekt griff) —
	// mutmaßlich Dirty-Region-Tracking, das eine nachträglich größer
	// werdende Fläche nicht neu zeichnet. Ein von Anfang an korrekt
	// dimensioniertes Bild braucht diese Kaskade gar nicht erst.
	if img, err := charts.DailyVolumeChart(nil); err == nil {
		v.dailyPanel.setImage(img)
	}
	if img, err := charts.TopSourcesChart(nil); err == nil {
		v.topPanel.setImage(img)
	}
	if img, err := charts.DispositionChart(nil); err == nil {
		v.dispPanel.setImage(img)
	}
	if img, err := charts.HeatmapChart(analysis.Heatmap{}); err == nil {
		v.heatPanel.setImage(img)
	}

	trendRow := container.NewHBox(v.tileTrendIcon, v.tileTrendLabel)

	v.content = container.NewVBox(
		tiles, trendRow,
		v.dailyPanel.container, v.topPanel.container, v.dispPanel.container, v.heatPanel.container,
	)

	v.scroll = container.NewVScroll(v.content)

	v.container = container.NewBorder(
		widget.NewLabelWithStyle(i18n.DashboardTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		v.scroll,
	)

	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer erfüllt fyne.Widget.
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.container)
}

// Reload lädt die Übersicht mit dem zuletzt per SetFilter gesetzten
// Filter neu — für den Aufruf nach einem Sync.
func (v *View) Reload() {
	v.load()
}

// SetFilter übernimmt Zeitraum und Domain aus der gemeinsamen
// Filterleiste (UMSETZUNGSPLAN.md AP-6-Checkliste: "wirkt auf alle
// Ansichten") und lädt neu.
func (v *View) SetFilter(period report.DateRange, domain string) {
	v.filterPeriod = period
	v.filterDomain = domain
	v.load()
}

func (v *View) load() {
	period, domain := v.filterPeriod, v.filterDomain
	v.runBackground(func() {
		dash, err := v.stats.Dashboard(context.Background(), analysis.Query{Period: period, Domain: domain})
		if err != nil {
			fyne.Do(func() { dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window) })
			return
		}

		dailyImg, err := v.charts.DailyVolumeChart(dash.DailyVolumes)
		if err == nil {
			var topImg, dispImg, heatImg image.Image
			topImg, err = v.charts.TopSourcesChart(dash.TopSources)
			if err == nil {
				dispImg, err = v.charts.DispositionChart(dash.Comparison.Current.VolumeByDisposition)
			}
			if err == nil {
				heatImg, err = v.charts.HeatmapChart(dash.Heatmap)
			}
			if err == nil {
				fyne.Do(func() { v.apply(dash, dailyImg, topImg, dispImg, heatImg) })
				return
			}
		}
		fyne.Do(func() { dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window) })
	})
}

func (v *View) apply(dash statistics.Dashboard, dailyImg, topImg, dispImg, heatImg image.Image) {
	stats := dash.Comparison.Current
	if stats.TotalMessages == 0 {
		v.setCenter(components.NewEmptyState(i18n.DashboardEmptyTitle, i18n.DashboardEmptyDetail, "", nil))
		return
	}
	v.setCenter(v.scroll)

	v.tileTotal.SetText(fmt.Sprintf("%d", stats.TotalMessages))
	v.tilePass.SetText(percent(stats.PassRate))
	v.tileDKIM.SetText(percent(stats.DKIMAlignmentRate))
	v.tileSPF.SetText(percent(stats.SPFAlignmentRate))
	v.tileSources.SetText(fmt.Sprintf("%d", stats.DistinctSources))
	v.tileTrendLabel.SetText(trendText(dash.Comparison))
	v.tileTrendIcon.SetResource(trendIcon(dash.Comparison))

	v.dailyPanel.setImage(dailyImg)
	v.topPanel.setImage(topImg)
	v.dispPanel.setImage(dispImg)
	v.heatPanel.setImage(heatImg)

	// Die Panel-Container wachsen durch setImage() auf die volle
	// Diagrammhöhe (siehe chartPanel.setImage), aber v.content (die VBox
	// aus Kacheln + allen vier Panels) und v.scroll bemerken das nicht von
	// selbst: container.Refresh() liest nur die bereits zugewiesene eigene
	// Size(), nicht die frisch gewachsenen Kind-Mindestgrößen. Ohne diese
	// beiden Refreshs bleibt v.scroll auf der alten, zu kleinen
	// Scrollfläche stehen und schneidet die unteren Diagrammbereiche
	// (Achsen-/IP-Beschriftungen) ab.
	v.content.Refresh()
	v.scroll.Refresh()
}

// setCenter tauscht den mittleren Border-Slot aus. NewBorder ordnet
// Objects als [center, top, ...] — der Center-Slot ist bei einem reinen
// Top-Border (kein Bottom/Left/Right) Index 0 (siehe AGENTS.md,
// Abschnitt zu container.NewBorder).
func (v *View) setCenter(obj fyne.CanvasObject) {
	v.container.Objects[0] = obj
	v.container.Refresh()
}

func percent(rate float64) string {
	return fmt.Sprintf("%.1f%%", rate*100)
}

// trendIcon liefert die Pfeil-Vektorgrafik zum Trend — ein Fyne-Icon statt
// eines Unicode-Pfeilzeichens im Label-Text: die von Fyne gebündelte
// Standardschrift deckt Pfeil-/Pfeilspitzen-Symbole ("→"/"▲"/"▼") nicht ab
// und zeigt ohne automatischen Font-Fallback stattdessen ein
// Ersatzzeichen ("�"). nil bedeutet "kein Pfeil anzeigen" (unverändert
// oder keine Vorperiode).
func trendIcon(c statistics.Comparison) fyne.Resource {
	if !c.HasPreviousPeriodData {
		return nil
	}
	switch {
	case c.PassRateTrend > 0.0005:
		return theme.MoveUpIcon()
	case c.PassRateTrend < -0.0005:
		return theme.MoveDownIcon()
	default:
		return nil
	}
}

func trendText(c statistics.Comparison) string {
	if !c.HasPreviousPeriodData {
		return i18n.DashboardTrendNoData
	}
	return fmt.Sprintf(i18n.DashboardTrendFmt, c.PassRateTrend*100)
}

// newTile baut eine Kennzahlen-Kachel. glossaryTerm verlinkt optional
// (leer = kein Knopf) auf eine Begriffserklärung — Ersatz für
// Hover-Tooltips, siehe internal/ui/glossary.
func newTile(title, glossaryTerm string, value *widget.Label, window fyne.Window) fyne.CanvasObject {
	value.TextStyle = fyne.TextStyle{Bold: true}

	var titleRow fyne.CanvasObject = widget.NewLabel(title)
	if glossaryTerm != "" {
		titleRow = container.NewHBox(widget.NewLabel(title), glossary.NewInfoButton(glossaryTerm, window))
	}
	return container.NewVBox(titleRow, value)
}

// exportChart baut den Callback für den PNG-Export-Knopf eines Diagramms
// — image liefert das zuletzt gerenderte Bild dieses Diagramms (nil,
// solange noch nichts geladen wurde).
func (v *View) exportChart(image func() image.Image) func() {
	return func() {
		img := image()
		if img == nil {
			return
		}
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowInformation(i18n.ExportFailedTitle, err.Error(), v.window)
				return
			}
			if writer == nil {
				return // Nutzer hat abgebrochen.
			}
			defer func() { _ = writer.Close() }()
			if err := exportdata.WriteChartPNG(writer, img); err != nil {
				dialog.ShowInformation(i18n.ExportFailedTitle, err.Error(), v.window)
			}
		}, v.window)
	}
}

// chartPanel ist ein Diagramm mit Titel und Export-Knopf.
type chartPanel struct {
	img       *canvas.Image
	current   image.Image
	container *fyne.Container
}

// newChartPanel baut ein Diagramm mit Titel, Erklär-Knopf ("?", siehe
// internal/ui/glossary) und Export-Knopf — title muss ein Begriff aus
// glossary.Terms sein (siehe glossary.terms.go).
func newChartPanel(title string, onExport func(), window fyne.Window) *chartPanel {
	p := &chartPanel{img: &canvas.Image{FillMode: canvas.ImageFillOriginal}}
	exportButton := widget.NewButton(i18n.DashboardExportChartPNG, onExport)
	titleRow := container.NewHBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		glossary.NewInfoButton(title, window),
	)
	p.container = container.NewVBox(
		titleRow,
		p.img,
		exportButton,
	)
	return p
}

func (p *chartPanel) setImage(img image.Image) {
	p.current = img
	p.img.Image = img
	p.img.Refresh()
	p.container.Refresh()
}

func (p *chartPanel) image() image.Image { return p.current }

// SetRunBackgroundForTest ersetzt die interne Hintergrund-Ausführung —
// für Tests aus anderen Paketen, die View einbetten (z. B. internal/ui).
// Nicht für Produktivcode gedacht, siehe AGENTS.md, Abschnitt zu
// Fyne-Tests.
func (v *View) SetRunBackgroundForTest(run func(func())) {
	v.runBackground = run
}
