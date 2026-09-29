package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// periodDayOptions are the selectable period presets — the same as
// formerly internal/ui/components.FilterBar (periodOptions), here without
// i18n label constants (internal/ui/i18n only moves to internal/web in a
// later step, see MIGRATIONSPLAN.md section 2).
var periodDayOptions = []int{7, 30, 90, 365}

// defaultPeriodDays is the default for the period filter when the
// request doesn't carry a (valid) "zeitraum" parameter.
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

// periodOptionView is a period option, ready-prepared for the <select>
// template (MIGRATIONSPLAN.md section 9/AGENTS.md addendum: preparation
// belongs in Go, not in the template).
type periodOptionView struct {
	Days     int
	Label    string
	Selected bool
}

// filterParams is the filter read from the URL (MIGRATIONSPLAN.md
// section 4 E-1/section 8: "filters live in the URL"). Applies equally to
// overview and chart endpoints — the same request URL yields the same
// period/domain for all three.
type filterParams struct {
	Days   int
	Domain string
}

// parseFilterParams reads "zeitraum" (days, only values from
// periodDayOptions) and "domain" from the request. A missing or invalid
// "zeitraum" value falls back to defaultPeriodDays instead of rejecting
// the request — a manipulated/stale link should still show a sensible
// overview.
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

// query builds the analysis.Query needed for repository queries from the
// filter — Period always ends "now", not at the time of the page view,
// which a user might remember.
func (f filterParams) query() (analysis.Query, error) {
	end := time.Now().UTC()
	begin := end.AddDate(0, 0, -f.Days)
	period, err := report.NewDateRange(begin, end)
	if err != nil {
		return analysis.Query{}, err
	}
	return analysis.Query{Period: period, Domain: f.Domain}, nil
}

// options returns the period selection list with the current selection
// marked, ready for dashboard.html.
func (f filterParams) options() []periodOptionView {
	out := make([]periodOptionView, len(periodDayOptions))
	for i, d := range periodDayOptions {
		out[i] = periodOptionView{Days: d, Label: periodLabel(d), Selected: d == f.Days}
	}
	return out
}
