package web

import (
	"fmt"
	"net/http"
)

// dashboardPageData holds the values layout.html/dashboard.html need —
// already fully formatted (percentages etc.), so the template doesn't
// need to contain formatting logic (MIGRATIONSPLAN.md section 9/AGENTS.md
// addendum: "preparation ... belongs in Go").
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

	// PeriodOptions and Domain fill the filter bar (MIGRATIONSPLAN.md
	// milestone M1: "filter bar with values in the URL").
	PeriodOptions []periodOptionView
	Domain        string
}

// handleDashboard renders the overview: metric tiles from
// statistics.UseCase.Dashboard, filtered by period/domain from the URL
// (M1). The page itself loads the two charts via JavaScript from
// /api/diagramme/* (section 6a) — they're deliberately not already in
// dashboardPageData.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
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
		Title:                "Overview",
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

	if err := s.views.render(w, r, "dashboard.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func formatPercent(rate float64) string {
	return fmt.Sprintf("%.1f %%", rate*100)
}

func formatTrend(passRateTrendPoints float64) string {
	return fmt.Sprintf("%+.1f percentage points vs. previous period", passRateTrendPoints*100)
}
