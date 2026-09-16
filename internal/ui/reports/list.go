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

	v.container = container.NewBorder(
		widget.NewLabelWithStyle(i18n.ReportsTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.loadMore, nil, nil,
		v.list,
	)

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
func (v *View) Reload() {
	v.data = nil
	v.nextCursor = ""
	v.loadPage()
}

func (v *View) loadMoreReports() {
	v.loadPage()
}

func (v *View) loadPage() {
	if v.loading {
		return
	}
	v.loading = true

	cursor := v.nextCursor
	v.runBackground(func() {
		page, err := v.queries.List(context.Background(), report.Query{
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
		empty := components.NewEmptyState(i18n.ReportsEmptyTitle, i18n.ReportsEmptyDetail, "", nil)
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
			d := dialog.NewCustomWithoutButtons(i18n.ReportDetailTitle, detail, v.window)
			d.Resize(fyne.NewSize(700, 500))
			d.Show()
		})
	})
}

// SetRunBackgroundForTest ersetzt die interne Hintergrund-Ausführung —
// für Tests aus anderen Paketen, die View einbetten (z. B. internal/ui).
// Nicht für Produktivcode gedacht, siehe AGENTS.md, Abschnitt zu
// Fyne-Tests.
func (v *View) SetRunBackgroundForTest(run func(func())) {
	v.runBackground = run
}
