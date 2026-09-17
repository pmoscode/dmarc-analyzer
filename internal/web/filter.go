package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// periodDayOptions sind die wählbaren Zeitraum-Voreinstellungen — dieselben
// wie zuvor internal/ui/components.FilterBar (periodOptions), hier ohne
// i18n-Label-Konstanten (internal/ui/i18n wandert erst in einem späteren
// Schritt nach internal/web, siehe MIGRATIONSPLAN.md Abschnitt 2).
var periodDayOptions = []int{7, 30, 90, 365}

// defaultPeriodDays ist die Vorbelegung des Zeitraum-Filters, wenn die
// Anfrage keinen (gültigen) "zeitraum"-Parameter mitbringt.
const defaultPeriodDays = 30

func periodLabel(days int) string {
	switch days {
	case 7:
		return "Letzte 7 Tage"
	case 30:
		return "Letzte 30 Tage"
	case 90:
		return "Letzte 90 Tage"
	case 365:
		return "Letztes Jahr"
	default:
		return "Letzte " + strconv.Itoa(days) + " Tage"
	}
}

func isAllowedPeriodDays(days int) bool {
	for _, d := range periodDayOptions {
		if d == days {
			return true
		}
	}
	return false
}

// periodOptionView ist eine Zeitraum-Option, fertig für die
// <select>-Vorlage aufbereitet (MIGRATIONSPLAN.md Abschnitt 9/
// AGENTS.md-Nachtrag: Aufbereitung gehört nach Go, nicht in die Vorlage).
type periodOptionView struct {
	Days     int
	Label    string
	Selected bool
}

// filterParams ist der aus der URL gelesene Filter (MIGRATIONSPLAN.md
// Abschnitt 4 E-1/Abschnitt 8: "Filter stehen in der URL"). Wirkt gleich
// auf Übersicht und Diagramm-Endpunkte — dieselbe Anfrage-URL liefert für
// alle drei denselben Zeitraum/dieselbe Domain.
type filterParams struct {
	Days   int
	Domain string
}

// parseFilterParams liest "zeitraum" (Tage, nur Werte aus
// periodDayOptions) und "domain" aus der Anfrage. Ein fehlender oder
// ungültiger "zeitraum"-Wert fällt auf defaultPeriodDays zurück, statt
// die Anfrage abzulehnen — ein manipulierter/veralteter Link soll
// weiterhin eine sinnvolle Übersicht zeigen.
func parseFilterParams(r *http.Request) filterParams {
	days := defaultPeriodDays
	if raw := r.URL.Query().Get("zeitraum"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && isAllowedPeriodDays(n) {
			days = n
		}
	}
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	return filterParams{Days: days, Domain: domain}
}

// query baut die für Repository-Abfragen nötige analysis.Query aus dem
// Filter — Period endet immer "jetzt", nicht beim Seitenaufruf, das sich
// ein Nutzer merken könnte.
func (f filterParams) query() (analysis.Query, error) {
	end := time.Now().UTC()
	begin := end.AddDate(0, 0, -f.Days)
	period, err := report.NewDateRange(begin, end)
	if err != nil {
		return analysis.Query{}, err
	}
	return analysis.Query{Period: period, Domain: f.Domain}, nil
}

// options liefert die Zeitraum-Auswahlliste mit markierter aktueller
// Auswahl, fertig für dashboard.html.
func (f filterParams) options() []periodOptionView {
	out := make([]periodOptionView, len(periodDayOptions))
	for i, d := range periodDayOptions {
		out[i] = periodOptionView{Days: d, Label: periodLabel(d), Selected: d == f.Days}
	}
	return out
}
