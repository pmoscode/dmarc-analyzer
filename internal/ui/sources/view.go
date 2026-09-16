// Package sources zeigt die nach Quell-IP aggregierte Sendequellen-
// Ansicht inklusive rDNS/PTR und Diensterkennung (IMPLEMENTIERUNG.md
// Abschnitt 10.1).
package sources

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/app/exportdata"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/components"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// pageSize ist die je Ladeschritt angeforderte Seitengröße — dieselbe
// Lazy-Nachladelogik wie internal/ui/reports.View.
const pageSize = 50

// View zeigt die nach Quell-IP aggregierte Sendequellen-Tabelle,
// seitenweise nachgeladen über sourcestats.UseCase.List (Keyset-
// Pagination, siehe sources.Page.NextCursor).
type View struct {
	widget.BaseWidget

	sources *sourcestats.UseCase
	window  fyne.Window
	// runBackground: siehe internal/ui/settings.View (AGENTS.md).
	runBackground func(f func())

	data       []domainsources.Stat
	nextCursor string
	loading    bool

	// filterPeriod/filterDomain: siehe internal/ui/reports.View.SetFilter.
	filterPeriod *report.DateRange
	filterDomain string

	list      *widget.List
	loadMore  *widget.Button
	container *fyne.Container
}

// NewView erzeugt die Sendequellen-Ansicht mit leerem Zustand. SetFilter()
// lädt die erste Seite — bewusst nicht automatisch im Konstruktor, siehe
// settings.NewView für dieselbe Begründung.
func NewView(sources *sourcestats.UseCase, window fyne.Window) *View {
	v := &View{
		sources:       sources,
		window:        window,
		runBackground: func(f func()) { go f() },
	}

	v.list = widget.NewList(
		func() int { return len(v.data) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		v.updateRow,
	)

	v.loadMore = widget.NewButton(i18n.SourcesLoadMore, v.loadMorePage)
	v.loadMore.Hide()

	exportButton := widget.NewButton(i18n.ExportCSV, v.exportCSV)

	v.container = container.NewBorder(
		container.NewHBox(widget.NewLabelWithStyle(i18n.SourcesTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), exportButton),
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
	s := v.data[id]

	hostname := s.Enrichment.Hostname
	if hostname == "" {
		hostname = i18n.SourcesUnknownValue
	}
	service := s.Enrichment.Service
	if service == "" {
		service = i18n.SourcesUnknownValue
	}

	obj.(*widget.Label).SetText(fmt.Sprintf("%s — %d Nachrichten, %.1f%% Pass-Rate, %s (%s)",
		s.SourceIP.String(), s.TotalCount, s.PassRate*100, hostname, service))
}

// Reload lädt die Sendequellen mit dem zuletzt per SetFilter gesetzten
// Filter neu — für den Aufruf nach einem Sync.
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

func (v *View) loadMorePage() {
	v.loadPage()
}

func (v *View) loadPage() {
	if v.loading {
		return
	}
	v.loading = true

	cursor := v.nextCursor
	period, domain := v.filterPeriod, v.filterDomain
	v.runBackground(func() {
		page, err := v.sources.List(context.Background(), domainsources.Query{
			Period: period, Domain: domain,
			Limit: pageSize, Cursor: cursor,
		})
		fyne.Do(func() {
			v.loading = false
			if err != nil {
				dialog.ShowInformation(i18n.ErrorGenericTitle, i18n.ErrorLoadFailed, v.window)
				return
			}
			v.data = append(v.data, page.Stats...)
			v.nextCursor = page.NextCursor
			v.refreshContent()
		})
	})
}

// refreshContent zeigt den Leerzustand, wenn (noch) keine Sendequellen
// vorliegen, sonst die Liste plus "Weitere laden", falls eine weitere
// Seite existiert (UMSETZUNGSPLAN.md Abschnitt 3.1: Leerzustände mit
// Handlungsaufforderung).
func (v *View) refreshContent() {
	if len(v.data) == 0 {
		empty := components.NewEmptyState(i18n.SourcesEmptyTitle, i18n.SourcesEmptyDetail, "", nil)
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
// Objects als [center, top, bottom, ...] — der Center-Slot ist bei einem
// reinen Top+Bottom-Border (kein Left/Right) Index 0 (siehe AGENTS.md,
// Abschnitt zu container.NewBorder).
func (v *View) setCenter(obj fyne.CanvasObject) {
	v.container.Objects[0] = obj
	v.container.Refresh()
}

// exportCSV exportiert die aktuell geladenen Sendequellen (siehe
// reports.View.exportCSV für dieselbe Begründung, warum das die sichtbare
// Seite und nicht zwangsläufig der gesamte gefilterte Bestand ist).
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
		if err := exportdata.WriteSourceStatsCSV(writer, data); err != nil {
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
