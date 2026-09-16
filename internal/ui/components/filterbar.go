package components

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
)

// periodOption ist ein wählbarer Zeitraum-Voreinstellung. Feste
// Voreinstellungen statt eines Datumspickers halten die Filterleiste
// einfach — ein beliebiges Von/Bis wäre ein sinnvolles v1.1-Polish, aber
// für "wirkt auf alle Ansichten" (UMSETZUNGSPLAN.md AP-6-Checkliste)
// reichen relative Zeiträume.
type periodOption struct {
	label string
	days  int
}

var periodOptions = []periodOption{
	{i18n.FilterPeriod7Days, 7},
	{i18n.FilterPeriod30Days, 30},
	{i18n.FilterPeriod90Days, 90},
	{i18n.FilterPeriod1Year, 365},
}

// defaultPeriodIndex ist die Voreinstellung beim ersten Anzeigen — 30
// Tage sind ein vernünftiger Standardzeitraum für ein Dashboard.
const defaultPeriodIndex = 1

// FilterBar bündelt Zeitraum- und Domain-Filter, die laut
// UMSETZUNGSPLAN.md AP 6 auf mehrere Ansichten gemeinsam wirken
// (Übersicht, Berichte, Sendequellen). Wandelt die Voreinstellung erst
// beim Anwenden in ein konkretes report.DateRange um ("jetzt" ändert
// sich), nicht schon bei der Auswahl.
type FilterBar struct {
	widget.BaseWidget

	period *widget.Select
	domain *widget.Entry
	apply  *widget.Button

	// OnChanged wird aufgerufen, wenn der Nutzer "Filter anwenden" tippt,
	// mit dem aktuell gewählten Zeitraum und (ggf. leeren) Domain-Filter.
	OnChanged func(period report.DateRange, domain string)

	container *fyne.Container
}

// NewFilterBar erzeugt eine einsatzbereite Filterleiste mit
// Standardauswahl (letzte 30 Tage, keine Domain-Einschränkung).
func NewFilterBar() *FilterBar {
	f := &FilterBar{}

	labels := make([]string, len(periodOptions))
	for i, o := range periodOptions {
		labels[i] = o.label
	}
	f.period = widget.NewSelect(labels, nil)
	f.period.SetSelectedIndex(defaultPeriodIndex)

	f.domain = widget.NewEntry()
	f.domain.SetPlaceHolder(i18n.FilterDomainHint)

	f.apply = widget.NewButton(i18n.FilterApply, f.notifyChanged)

	f.container = container.NewHBox(
		widget.NewLabel(i18n.FilterPeriodLabel), f.period,
		widget.NewLabel(i18n.FilterDomainLabel), f.domain,
		f.apply,
	)

	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer erfüllt fyne.Widget.
func (f *FilterBar) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(f.container)
}

// CurrentPeriod berechnet den aktuell in der Leiste gewählten Zeitraum
// (relativ zu "jetzt"), ohne dass der Nutzer "Filter anwenden" tippen
// muss — für den initialen Ladevorgang einer Ansicht.
func (f *FilterBar) CurrentPeriod() report.DateRange {
	days := periodOptions[defaultPeriodIndex].days
	for _, o := range periodOptions {
		if o.label == f.period.Selected {
			days = o.days
			break
		}
	}

	end := time.Now().UTC()
	begin := end.Add(-time.Duration(days) * 24 * time.Hour)
	// NewDateRange schlägt nur bei Begin >= End fehl — bei days > 0 kann
	// das hier nie passieren.
	period, _ := report.NewDateRange(begin, end)
	return period
}

// CurrentDomain liefert den aktuell eingegebenen Domain-Filter, ohne
// führende/folgende Leerzeichen.
func (f *FilterBar) CurrentDomain() string {
	return strings.TrimSpace(f.domain.Text)
}

func (f *FilterBar) notifyChanged() {
	if f.OnChanged != nil {
		f.OnChanged(f.CurrentPeriod(), f.CurrentDomain())
	}
}
