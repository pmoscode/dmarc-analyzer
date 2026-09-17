// Package reports zeigt die virtualisierte Berichtstabelle und die
// Bericht-Detailansicht (IMPLEMENTIERUNG.md Abschnitt 10.1).
package reports

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// pageSize ist die je Ladeschritt angeforderte Seitengröße — die
// Lazy-Datenquelle lädt nie alle Reports auf einmal
// (IMPLEMENTIERUNG.md Abschnitt 10.4).
const pageSize = 50

// View zeigt die Berichtstabelle: eine Zeile pro Report, seitenweise
// nachgeladen über queryreports.UseCase.List (Keyset-Pagination, siehe
// report.Page.NextCursor). Antippen einer Zeile öffnet die Detailansicht.
type View struct {
	widget.BaseWidget

	queries *queryreports.UseCase
	window  fyne.Window
	// runBackground: siehe internal/ui/settings.View — dieselbe
	// Testbarkeits-Begründung (AGENTS.md).
	runBackground func(f func())

	data       []report.AggregateReport
	nextCursor string
	loading    bool

	// filterPeriod/filterDomain kommen von der gemeinsamen Filterleiste
	// (components.FilterBar) im Hauptfenster — siehe SetFilter. nil
	// filterPeriod bedeutet: kein Zeitraum-Filter.
	filterPeriod *report.DateRange
	filterDomain string

	// groupBy ist ansichtseigen (nicht Teil der gemeinsamen Filterleiste)
	// — Gruppierung nach Domain/Organisation ist ein Konzept der
	// Berichtstabelle, das sich auf Übersicht/Sendequellen nicht
	// überträgt (deren Ports kennen kein GroupBy).
	groupBy report.GroupBy
	group   *widget.Select

	list      *widget.List
	loadMore  *widget.Button
	container *fyne.Container
}

// NewView erzeugt die Berichtsansicht mit leerem Zustand. Reload() lädt
// die erste Seite — bewusst nicht automatisch im Konstruktor, siehe
// settings.NewView für dieselbe Begründung.
func NewView(queries *queryreports.UseCase, window fyne.Window) *View {
	v := &View{
		queries:       queries,
		window:        window,
		runBackground: func(f func()) { go f() },
	}

	v.list = widget.NewList(
		func() int { return len(v.data) },
		func() fyne.CanvasObject {
			return widget.NewButton("", nil)
		},
		v.updateRow,
	)

	v.loadMore = widget.NewButton(i18n.ReportsLoadMore, v.loadMoreReports)
	v.loadMore.Hide()

	exportButton := widget.NewButton(i18n.ExportCSV, v.exportCSV)

	// SetSelected löst OnChanged synchron aus (widget.Select) — deshalb
	// hier zunächst ohne Callback konstruieren und OnChanged erst setzen,
	// nachdem v.container existiert; sonst würde v.groupSelected() über
	// Reload()/setCenter() auf v.container zugreifen, bevor es zugewiesen
	// ist (Nil-Pointer).
	groupLabels := []string{i18n.ReportsGroupNone, i18n.ReportsGroupDomain, i18n.ReportsGroupOrg}
	v.group = widget.NewSelect(groupLabels, nil)
	v.group.SetSelected(i18n.ReportsGroupNone)

	v.container = container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle(i18n.ReportsTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			exportButton, widget.NewLabel(i18n.ReportsGroupLabel), v.group,
		),
		v.loadMore, nil, nil,
		v.list,
	)
	v.group.OnChanged = v.groupSelected

	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer erfüllt fyne.Widget.
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.container)
}

func (v *View) updateRow(id widget.ListItemID, obj fyne.CanvasObject) {
	if id < 0 || id >= len(v.data) {
		return
	}
	r := v.data[id]

	btn := obj.(*widget.Button)
	btn.SetText(fmt.Sprintf("%s — %s (%s bis %s)",
		r.Metadata.OrgName, r.Policy.Domain.String(),
		r.Metadata.Range.Begin.Format("2006-01-02"), r.Metadata.Range.End.Format("2006-01-02")))
	btn.OnTapped = func() { v.showDetail(r) }
}

// Reload verwirft den aktuellen Stand und lädt die erste Seite neu — für
// den Aufruf nach einem Sync oder beim ersten Anzeigen der Ansicht.
// Verwendet weiterhin den zuletzt per SetFilter gesetzten Filter.
func (v *View) Reload() {
	v.data = nil
	v.nextCursor = ""
	v.loadPage()
}

// SetFilter übernimmt Zeitraum und Domain aus der gemeinsamen
// Filterleiste (UMSETZUNGSPLAN.md AP-6-Checkliste: "wirkt auf alle
// Ansichten") und lädt die erste Seite neu. period nil bedeutet: kein
// Zeitraum-Filter.
func (v *View) SetFilter(period *report.DateRange, domain string) {
	v.filterPeriod = period
	v.filterDomain = domain
	v.Reload()
}

func (v *View) loadMoreReports() {
	v.loadPage()
}

func (v *View) groupSelected(label string) {
	switch label {
	case i18n.ReportsGroupDomain:
		v.groupBy = report.GroupByDomain
	case i18n.ReportsGroupOrg:
		v.groupBy = report.GroupByOrg
	default:
		v.groupBy = report.GroupByNone
	}
	v.Reload()
}

func (v *View) loadPage() {
	if v.loading {
		return
	}
	v.loading = true

	cursor := v.nextCursor
	period, domain, groupBy := v.filterPeriod, v.filterDomain, v.groupBy
	v.runBackground(func() {
		page, err := v.queries.List(context.Background(), report.Query{
			Period: period, Domain: domain, GroupBy: groupBy,
			SortField: report.SortByDateBegin, SortDirection: report.SortDescending,
			Limit: pageSize, Cursor: cursor,
		})
		fyne.Do(func() {
			v.loading = false
			if err != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window)
				return
			}
			v.data = append(v.data, page.Reports...)
			v.nextCursor = page.NextCursor
			v.refreshContent()
		})
	})
}

// refreshContent zeigt den Leerzustand, wenn (noch) keine Reports
// vorliegen, sonst die Liste plus "Weitere laden", falls eine weitere
// Seite existiert (UMSETZUNGSPLAN.md Abschnitt 3.1: Leerzustände mit
// Handlungsaufforderung).
func (v *View) refreshContent() {
	if len(v.data) == 0 {
		detail := i18n.ReportsEmptyDetail
		if v.filterPeriod != nil || v.filterDomain != "" {
			detail = i18n.ReportsEmptyNoMatch
		}
		empty := components.NewEmptyState(i18n.ReportsEmptyTitle, detail, "", nil)
		v.setCenter(empty)
		v.loadMore.Hide()
		return
	}

	v.setCenter(v.list)
	v.list.Refresh()
	if v.nextCursor != "" {
		v.loadMore.Show()
	} else {
		v.loadMore.Hide()
	}
}

// setCenter tauscht den mittleren Border-Slot aus. NewBorder ordnet
// Objects als [center, top, bottom, left, right, ...] — der Center-Slot
// ist bei einem reinen Top+Bottom-Border (kein Left/Right) Index 0.
func (v *View) setCenter(obj fyne.CanvasObject) {
	v.container.Objects[0] = obj
	v.container.Refresh()
}

func (v *View) showDetail(r report.AggregateReport) {
	progress := dialog.NewCustomWithoutButtons(i18n.ReportDetailTitle,
		container.NewVBox(widget.NewLabel(""), widget.NewProgressBarInfinite()), v.window)
	progress.Show()

	v.runBackground(func() {
		full, err := v.queries.Get(context.Background(), r.ID)
		fyne.Do(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window)
				return
			}
			detail := NewDetailView(full)
			// NewCustomWithoutButtons hier ein echter Bug (nicht nur
			// Test-Rauschen): der Dialog läuft auf einem
			// widget.NewModalPopUp, der anders als ein gewöhnliches Popup
			// NICHT durch Antippen außerhalb schließt — ohne Knopf und
			// ohne jeden Code-Pfad, der Hide() aufruft, blieb der
			// Bericht-Detaildialog für den Nutzer dauerhaft offen.
			d := dialog.NewCustom(i18n.ReportDetailTitle, i18n.ButtonClose, detail, v.window)
			d.Resize(fyne.NewSize(700, 500))
			d.Show()
		})
	})
}

// exportCSV exportiert die aktuell geladenen Reports (also die sichtbare,
// gefilterte Seite, nicht zwangsläufig jeden Report, der dem Filter
// insgesamt entspricht — Laden aller Seiten nur für den Export wäre bei
// großen Beständen selbst eine Performance-Falle, siehe
// UMSETZUNGSPLAN.md Abschnitt 3.3 zur Lazy-Datenquelle) als CSV
// (FEATURES.md Vorschlag 11.4).
func (v *View) exportCSV() {
	data := v.data
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowInformation(i18n.ExportFailedTitle, err.Error(), v.window)
			return
		}
		if writer == nil {
			return // Nutzer hat abgebrochen.
		}
		defer func() { _ = writer.Close() }()
		if err := exportdata.WriteReportsCSV(writer, data); err != nil {
			dialog.ShowInformation(i18n.ExportFailedTitle, err.Error(), v.window)
		}
	}, v.window)
}

// SetRunBackgroundForTest ersetzt die interne Hintergrund-Ausführung —
// für Tests aus anderen Paketen, die View einbetten (z. B. internal/ui).
// Nicht für Produktivcode gedacht, siehe AGENTS.md, Abschnitt zu
// Fyne-Tests.
func (v *View) SetRunBackgroundForTest(run func(func())) {
	v.runBackground = run
}
