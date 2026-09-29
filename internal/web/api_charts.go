package web

import (
	"net/http"
	"net/url"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// dayLabel is the short format shown in the chart ("01.09."), dayISO the
// one for drill-down URLs and as a unique matrix key (MIGRATIONSPLAN.md
// section 6a: "the server only delivers data ... target URL for the
// drill-down").
func dayLabel(t time.Time) string { return t.Format("02.01.") }
func dayISO(t time.Time) string   { return t.Format("2006-01-02") }

// reportsURL builds the target URL for the drill-down from a chart point
// to the (not yet existing, milestone M2) reports table — fully prepared
// by the server, so charts.js doesn't need to know its own filter logic.
// domainFilter is the domain filter currently set in the filter bar
// (empty: none) — a drill-down shouldn't lose the filter the chart was
// drawn under (MIGRATIONSPLAN.md section 9.6).
func reportsURL(day time.Time, sourceIP string, domainFilter string) string {
	v := url.Values{}
	v.Set("von", dayISO(day))
	v.Set("bis", dayISO(day.AddDate(0, 0, 1)))
	if sourceIP != "" {
		v.Set("quelle", sourceIP)
	}
	if domainFilter != "" {
		v.Set("domain", domainFilter)
	}
	return "/berichte?" + v.Encode()
}

// reportsURLForPeriod is the same drill-down URL as reportsURL, but for
// the entire selected period instead of a single day — for top sending
// sources (whole period, filtered to one IP) and the disposition
// breakdown (whole period, filtered to one disposition). disposition is
// empty when no disposition filter should be set.
func reportsURLForPeriod(period report.DateRange, sourceIP, domainFilter string, disposition report.Disposition) string {
	v := url.Values{}
	v.Set("von", dayISO(period.Begin))
	v.Set("bis", dayISO(period.End))
	if sourceIP != "" {
		v.Set("quelle", sourceIP)
	}
	if domainFilter != "" {
		v.Set("domain", domainFilter)
	}
	if disposition != "" {
		v.Set("disposition", string(disposition))
	}
	return "/berichte?" + v.Encode()
}

// --- Message volume per day (stacked bar chart) -------------------------

type dailyVolumeResponse struct {
	Days []dailyVolumePoint `json:"days"`
}

type dailyVolumePoint struct {
	Label string `json:"label"`
	Pass  int    `json:"pass"`
	Fail  int    `json:"fail"`
	URL   string `json:"url"`
}

func (s *Server) handleChartDailyVolume(w http.ResponseWriter, r *http.Request) {
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

	resp := dailyVolumeResponse{Days: make([]dailyVolumePoint, len(dash.DailyVolumes))}
	for i, d := range dash.DailyVolumes {
		resp.Days[i] = dailyVolumePoint{
			Label: dayLabel(d.Day),
			Pass:  d.Pass,
			Fail:  d.Fail,
			URL:   reportsURL(d.Day, "", filter.Domain),
		}
	}

	s.writeJSON(w, r, resp)
}

// --- Sending source × day (heatmap) -------------------------------------

type heatmapResponse struct {
	SourceLabels []string      `json:"sourceLabels"`
	DayLabels    []string      `json:"dayLabels"`
	Cells        []heatmapCell `json:"cells"`
}

// heatmapCell carries X/Y as axis labels (chartjs-chart-matrix maps cells
// on a category axis via exactly these strings, see charts.js) instead
// of via a row/column index.
type heatmapCell struct {
	X        string  `json:"x"`
	Y        string  `json:"y"`
	PassRate float64 `json:"passRate"`
	Total    int     `json:"total"`
	HasData  bool    `json:"hasData"`
	URL      string  `json:"url"`
}

func (s *Server) handleChartHeatmap(w http.ResponseWriter, r *http.Request) {
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

	heatmap := dash.Heatmap
	resp := heatmapResponse{
		SourceLabels: make([]string, len(heatmap.Sources)),
		DayLabels:    make([]string, len(heatmap.Days)),
	}
	for i, day := range heatmap.Days {
		resp.DayLabels[i] = dayLabel(day)
	}
	for si, ip := range heatmap.Sources {
		label := ip.String()
		if si < len(heatmap.SourceLabels) && heatmap.SourceLabels[si] != "" {
			// Always show the IP address too, not just the detected name:
			// chartjs-chart-matrix maps cells on a category axis via the
			// label string (see charts.js) — two different sources with
			// the same detected service (e.g. two IPs from "Google
			// Workspace") would otherwise wrongly end up in the same row.
			label = heatmap.SourceLabels[si] + " (" + ip.String() + ")"
		}
		resp.SourceLabels[si] = label

		for di, day := range heatmap.Days {
			cell := heatmap.Cells[si][di]
			resp.Cells = append(resp.Cells, heatmapCell{
				X:        resp.DayLabels[di],
				Y:        label,
				PassRate: cell.PassRate,
				Total:    cell.Total,
				HasData:  cell.HasData,
				URL:      reportsURL(day, ip.String(), filter.Domain),
			})
		}
	}

	s.writeJSON(w, r, resp)
}

// --- Top sending sources (horizontal bar chart) --------------------------

type sourceVolumeResponse struct {
	Sources []sourceVolumePoint `json:"sources"`
}

type sourceVolumePoint struct {
	Label    string  `json:"label"`
	Total    int     `json:"total"`
	PassRate float64 `json:"passRate"`
	URL      string  `json:"url"`
}

// sourceLabel shows the name enriched by statistics.UseCase (detected
// service or PTR hostname) in addition to the IP address — analogous to
// the heatmap row labels above (same reasoning: two sources with the same
// detected service must stay distinguishable).
func sourceLabel(s report.SourceIP, enrichedLabel string) string {
	if enrichedLabel == "" {
		return s.String()
	}
	return enrichedLabel + " (" + s.String() + ")"
}

func (s *Server) handleChartTopSources(w http.ResponseWriter, r *http.Request) {
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

	resp := sourceVolumeResponse{Sources: make([]sourceVolumePoint, len(dash.TopSources))}
	for i, src := range dash.TopSources {
		resp.Sources[i] = sourceVolumePoint{
			Label:    sourceLabel(src.SourceIP, src.Label),
			Total:    src.Total,
			PassRate: src.PassRate,
			URL:      reportsURLForPeriod(q.Period, src.SourceIP.String(), filter.Domain, ""),
		}
	}

	s.writeJSON(w, r, resp)
}

// --- Disposition breakdown (donut) ----------------------------------------

// dispositionOrder fixes a deterministic order for the donut — the same
// order as formerly internal/infra/charts (Renderer.DispositionChart);
// iterating over a map wouldn't be reproducible.
var dispositionOrder = []report.Disposition{
	report.DispositionNone, report.DispositionQuarantine, report.DispositionReject, report.DispositionUnknown,
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

type dispositionResponse struct {
	Slices []dispositionSlice `json:"slices"`
}

type dispositionSlice struct {
	Disposition string `json:"disposition"`
	Label       string `json:"label"`
	Total       int    `json:"total"`
	URL         string `json:"url"`
}

func (s *Server) handleChartDisposition(w http.ResponseWriter, r *http.Request) {
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

	byDisposition := dash.Comparison.Current.VolumeByDisposition
	resp := dispositionResponse{Slices: make([]dispositionSlice, 0, len(dispositionOrder))}
	for _, d := range dispositionOrder {
		resp.Slices = append(resp.Slices, dispositionSlice{
			Disposition: string(d),
			Label:       dispositionLabel(d),
			Total:       byDisposition[d],
			URL:         reportsURLForPeriod(q.Period, "", filter.Domain, d),
		})
	}

	s.writeJSON(w, r, resp)
}
