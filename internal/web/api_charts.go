package web

import (
	"net/http"
	"net/url"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// dayLabel ist das im Diagramm angezeigte Kurzformat ("01.09."), dayISO
// das für Drill-down-URLs und als eindeutiger Matrix-Schlüssel
// (MIGRATIONSPLAN.md Abschnitt 6a: "Der Server liefert nur Daten ...
// Ziel-URL für den Drill-down").
func dayLabel(t time.Time) string { return t.Format("02.01.") }
func dayISO(t time.Time) string   { return t.Format("2006-01-02") }

// reportsURL baut die Ziel-URL für den Drill-down von einem Diagrammpunkt
// zur (noch nicht existierenden, Meilenstein M2) Berichtstabelle —
// fertig aufbereitet vom Server, damit charts.js keine eigene
// Filterlogik kennen muss. domainFilter ist der aktuell in der
// Filterleiste gesetzte Domain-Filter (leer: keiner) — ein Drill-down
// soll den Filter nicht verlieren, unter dem das Diagramm gezeichnet
// wurde (MIGRATIONSPLAN.md Abschnitt 9.6).
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

// reportsURLForPeriod ist dieselbe Drill-down-URL wie reportsURL, aber für
// den gesamten gewählten Zeitraum statt für einen einzelnen Tag — für
// Top-Sendequellen (ganzer Zeitraum, gefiltert auf eine IP) und die
// Disposition-Verteilung (ganzer Zeitraum, gefiltert auf eine
// Disposition). disposition ist leer, wenn kein Disposition-Filter
// gesetzt werden soll.
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

// --- Nachrichtenvolumen pro Tag (gestapeltes Balkendiagramm) -----------

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

// --- Sendequelle × Tag (Heatmap) ----------------------------------------

type heatmapResponse struct {
	SourceLabels []string      `json:"sourceLabels"`
	DayLabels    []string      `json:"dayLabels"`
	Cells        []heatmapCell `json:"cells"`
}

// heatmapCell trägt X/Y als Achsen-Label (chartjs-chart-matrix ordnet
// Zellen bei einer category-Achse über exakt diese Strings zu, siehe
// charts.js) statt über einen Zeilen-/Spaltenindex.
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
			// IP-Adresse immer mit anzeigen, nicht nur den erkannten Namen:
			// chartjs-chart-matrix ordnet Zellen bei einer category-Achse
			// über den Label-String zu (siehe charts.js) — zwei
			// unterschiedliche Quellen mit demselben erkannten Dienst
			// (z. B. zwei IPs von "Google Workspace") dürften sonst
			// fälschlich in derselben Zeile landen.
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

// --- Top-Sendequellen (horizontales Balkendiagramm) ---------------------

type sourceVolumeResponse struct {
	Sources []sourceVolumePoint `json:"sources"`
}

type sourceVolumePoint struct {
	Label    string  `json:"label"`
	Total    int     `json:"total"`
	PassRate float64 `json:"passRate"`
	URL      string  `json:"url"`
}

// sourceLabel zeigt den von statistics.UseCase angereicherten Namen
// (erkannter Dienst oder PTR-Hostname) zusätzlich zur IP-Adresse — analog
// zu den Heatmap-Zeilenbeschriftungen oben (dieselbe Begründung: zwei
// Quellen mit demselben erkannten Dienst müssen unterscheidbar bleiben).
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

// --- Verteilung nach Disposition (Donut) --------------------------------

// dispositionOrder legt eine feste, deterministische Reihenfolge für den
// Donut fest — dieselbe Reihenfolge wie zuvor internal/infra/charts
// (Renderer.DispositionChart), die Iteration über eine map wäre nicht
// reproduzierbar.
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
