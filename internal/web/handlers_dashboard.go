package web

import (
	"fmt"
	"net/http"
)

// dashboardPageData sind die Werte, die layout.html/dashboard.html
// brauchen — bereits fertig formatiert (Prozentangaben etc.), damit die
// Vorlage keine Formatierungslogik enthalten muss (MIGRATIONSPLAN.md
// Abschnitt 9/AGENTS.md-Nachtrag: "Aufbereitung ... gehört nach Go").
type dashboardPageData struct {
	Title string
	Nav   []navItem

	TotalMessages        int
	PassRatePercent      string
	DKIMAlignmentPercent string
	SPFAlignmentPercent  string
	DistinctSources      int

	HasTrend  bool
	TrendUp   bool
	TrendText string

	// PeriodOptions und Domain füllen die Filterleiste (MIGRATIONSPLAN.md
	// Meilenstein M1: "Filterleiste mit Werten in der URL").
	PeriodOptions []periodOptionView
	Domain        string
}

// handleDashboard rendert die Übersicht: Kennzahlen-Kacheln aus
// statistics.UseCase.Dashboard, gefiltert nach Zeitraum/Domain aus der
// URL (M1). Die zwei Diagramme lädt die Seite selbst per JavaScript von
// /api/diagramme/* nach (Abschnitt 6a) — sie stehen bewusst nicht schon
// in dashboardPageData.
//
// "/" ist außerdem die einzige Stelle, die auf Ersteinrichtung bzw.
// Entsperren umleitet (MIGRATIONSPLAN.md Abschnitt 3: "Keine Konten
// vorhanden → / leitet auf die Ersteinrichtung um" / "Kein
// Betriebssystem-Schlüsselbund → Oberfläche zeigt zuerst eine
// Entsperr-Seite") — andere Seiten (Berichte, Sendequellen, …)
// funktionieren unabhängig von Konten/Schlüsselspeicher (sie lesen nur
// bereits importierte Daten), eine Umleitung dort wäre unnötig
// einschränkend.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if s.deps.Accounts != nil {
		accounts, err := s.deps.Accounts.List(r.Context())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if len(accounts) == 0 {
			http.Redirect(w, r, "/einrichtung", http.StatusSeeOther)
			return
		}
		if s.credentialsLocked() {
			s.redirectToUnlock(w, r, "/")
			return
		}
	}

	filter := parseFilterParams(r)
	q, err := filter.query()
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	dash, err := s.deps.Statistics.Dashboard(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	current := dash.Comparison.Current
	data := dashboardPageData{
		Title:                "Übersicht",
		Nav:                  navItems(r.URL.Path),
		TotalMessages:        current.TotalMessages,
		PassRatePercent:      formatPercent(current.PassRate),
		DKIMAlignmentPercent: formatPercent(current.DKIMAlignmentRate),
		SPFAlignmentPercent:  formatPercent(current.SPFAlignmentRate),
		DistinctSources:      current.DistinctSources,
		HasTrend:             dash.Comparison.HasPreviousPeriodData,
		TrendUp:              dash.Comparison.PassRateTrend >= 0,
		TrendText:            formatTrend(dash.Comparison.PassRateTrend),
		PeriodOptions:        filter.options(),
		Domain:               filter.Domain,
	}

	if err := s.views.render(w, "dashboard.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func formatPercent(rate float64) string {
	return fmt.Sprintf("%.1f %%", rate*100)
}

func formatTrend(passRateTrendPoints float64) string {
	return fmt.Sprintf("%+.1f Prozentpunkte ggü. Vorperiode", passRateTrendPoints*100)
}
