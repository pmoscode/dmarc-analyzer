package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// reportsPageSize ist die je Ladeschritt angeforderte Seitengröße —
// dieselbe Lazy-Nachladelogik wie zuvor internal/ui/reports.View
// (pageSize), hier über htmx statt über einen Fyne-Button (Abschnitt
// 7: "GET /berichte/seite | nächste Seite (htmx, Keyset-Cursor)").
const reportsPageSize = 50

// reportsFilter fasst alle Filter-/Sortier-/Gruppierungsparameter von
// /berichte zusammen — aus der URL gelesen: entweder der geteilten
// Filterleiste (zeitraum/domain, wie Übersicht) oder den absoluten
// Drill-down-Parametern eines Diagramm-Klicks (von/bis/quelle/
// disposition, MIGRATIONSPLAN.md Abschnitt 9.6).
type reportsFilter struct {
	Period       report.DateRange
	UsesAbsolute bool
	PeriodDays   int // nur gültig, wenn !UsesAbsolute

	Domain      string
	SourceIP    string
	Disposition report.Disposition // "" bedeutet: kein Filter

	GroupBy       report.GroupBy
	SortField     report.SortField
	SortDirection report.SortDirection
}

func parseReportsFilter(r *http.Request) (reportsFilter, error) {
	v := r.URL.Query()

	f := reportsFilter{
		Domain:        strings.TrimSpace(v.Get("domain")),
		SourceIP:      strings.TrimSpace(v.Get("quelle")),
		GroupBy:       parseGroupBy(v.Get("gruppierung")),
		SortField:     parseSortField(v.Get("sortierung")),
		SortDirection: parseSortDirection(v.Get("sortrichtung")),
	}

	if raw := strings.TrimSpace(v.Get("disposition")); raw != "" {
		f.Disposition = report.ParseDisposition(raw)
	}

	if von, bis, ok := parseISODateRange(v.Get("von"), v.Get("bis")); ok {
		period, err := report.NewDateRange(von, bis)
		if err != nil {
			return reportsFilter{}, err
		}
		f.Period = period
		f.UsesAbsolute = true
		return f, nil
	}

	fp := parseFilterParams(r)
	q, err := fp.query()
	if err != nil {
		return reportsFilter{}, err
	}
	f.Period = q.Period
	f.PeriodDays = fp.Days
	return f, nil
}

func parseISODateRange(vonRaw, bisRaw string) (time.Time, time.Time, bool) {
	if vonRaw == "" || bisRaw == "" {
		return time.Time{}, time.Time{}, false
	}
	von, errVon := time.Parse("2006-01-02", vonRaw)
	bis, errBis := time.Parse("2006-01-02", bisRaw)
	if errVon != nil || errBis != nil {
		return time.Time{}, time.Time{}, false
	}
	return von.UTC(), bis.UTC(), true
}

// parseGroupBy/parseSortField/parseSortDirection akzeptieren nur genau
// die von report.Query unterstützten Werte (deren Konstanten-Strings
// zufällig identisch zu den hier verwendeten Query-Parameter-Werten
// sind, siehe report.GroupBy/SortField/SortDirection) — jeder andere
// Wert (auch ein manipulierter Link) fällt auf die jeweilige
// Voreinstellung zurück, statt die Anfrage abzulehnen.
func parseGroupBy(raw string) report.GroupBy {
	switch report.GroupBy(raw) {
	case report.GroupByDomain, report.GroupByOrg:
		return report.GroupBy(raw)
	default:
		return report.GroupByNone
	}
}

func parseSortField(raw string) report.SortField {
	switch report.SortField(raw) {
	case report.SortByOrgName, report.SortByDomain:
		return report.SortField(raw)
	default:
		return report.SortByDateBegin
	}
}

func parseSortDirection(raw string) report.SortDirection {
	if report.SortDirection(raw) == report.SortAscending {
		return report.SortAscending
	}
	return report.SortDescending
}

// query baut die Repository-Abfrage für diesen Filter mit dem
// angegebenen Keyset-Cursor (leer für die erste Seite) und der
// angegebenen Seitengröße — für die Berichtstabelle selbst immer
// reportsPageSize, für den CSV-Export eine größere Seitengröße
// (handlers_export.go: weniger Datenbank-Roundtrips, ohne den gesamten
// gefilterten Bestand auf einmal zu laden).
func (f reportsFilter) query(cursor string, limit int) report.Query {
	period := f.Period
	return report.Query{
		Period:        &period,
		Domain:        f.Domain,
		SourceIP:      f.SourceIP,
		Disposition:   f.Disposition,
		SortField:     f.SortField,
		SortDirection: f.SortDirection,
		GroupBy:       f.GroupBy,
		Limit:         limit,
		Cursor:        cursor,
	}
}

// values baut die Query-Parameter, die den aktuellen Filter vollständig
// beschreiben — Grundlage für "Weitere laden" (mit zusätzlichem Cursor)
// und die sortierbaren Spaltenköpfe (mit geändertem
// sortierung/sortrichtung), damit keiner der beiden den Rest des Filters
// verliert.
func (f reportsFilter) values() url.Values {
	v := url.Values{}
	if f.UsesAbsolute {
		v.Set("von", dayISO(f.Period.Begin))
		v.Set("bis", dayISO(f.Period.End))
	} else {
		v.Set("zeitraum", strconv.Itoa(f.PeriodDays))
	}
	if f.Domain != "" {
		v.Set("domain", f.Domain)
	}
	if f.SourceIP != "" {
		v.Set("quelle", f.SourceIP)
	}
	if f.Disposition != "" {
		v.Set("disposition", string(f.Disposition))
	}
	if f.GroupBy != report.GroupByNone {
		v.Set("gruppierung", string(f.GroupBy))
	}
	v.Set("sortierung", string(f.SortField))
	v.Set("sortrichtung", string(f.SortDirection))
	return v
}

// sortLink baut den Link für einen sortierbaren Spaltenkopf: sortiert
// nach field, in umgekehrter Richtung, wenn field bereits die aktive
// Sortierung ist, sonst aufsteigend.
func (f reportsFilter) sortLink(field report.SortField) string {
	next := f
	if f.SortField == field {
		if f.SortDirection == report.SortAscending {
			next.SortDirection = report.SortDescending
		} else {
			next.SortDirection = report.SortAscending
		}
	} else {
		next.SortField = field
		next.SortDirection = report.SortAscending
	}
	return "/berichte?" + next.values().Encode()
}

func (f reportsFilter) sortIndicator(field report.SortField) string {
	if f.SortField != field {
		return ""
	}
	if f.SortDirection == report.SortAscending {
		return " ▲"
	}
	return " ▼"
}

// --- Vorlagendaten -------------------------------------------------------

type reportRowView struct {
	ID          string
	OrgName     string
	Domain      string
	PeriodLabel string
}

type reportsRowsData struct {
	Rows        []reportRowView
	HasMore     bool
	NextPageURL string
}

type dispositionOptionView struct {
	Value    string
	Label    string
	Selected bool
}

type groupOptionView struct {
	Value    string
	Label    string
	Selected bool
}

type reportsPageData struct {
	Title string
	Nav   []navItem

	UsesAbsolutePeriod bool
	PeriodFromLabel    string
	PeriodToLabel      string
	PeriodOptions      []periodOptionView
	Domain             string
	SourceIP           string
	DispositionOptions []dispositionOptionView
	GroupOptions       []groupOptionView

	SortOrgURL          string
	SortOrgIndicator    string
	SortDomainURL       string
	SortDomainIndicator string
	SortPeriodURL       string
	SortPeriodIndicator string

	// ExportURL exportiert den GESAMTEN aktuell gefilterten Bestand als
	// CSV (nicht nur die geladene Seite, siehe handlers_export.go) —
	// derselbe Filter wie die Tabelle gerade zeigt.
	ExportURL string

	Rows reportsRowsData
}

func reportRows(reports []report.AggregateReport) []reportRowView {
	rows := make([]reportRowView, len(reports))
	for i, rep := range reports {
		rows[i] = reportRowView{
			ID:      strconv.FormatInt(int64(rep.ID), 10),
			OrgName: rep.Metadata.OrgName,
			Domain:  rep.Policy.Domain.String(),
			PeriodLabel: rep.Metadata.Range.Begin.Format("2006-01-02") + " – " +
				rep.Metadata.Range.End.Format("2006-01-02"),
		}
	}
	return rows
}

func buildReportsPageData(filter reportsFilter, page report.Page) reportsPageData {
	data := reportsPageData{
		Title:               "Berichte",
		UsesAbsolutePeriod:  filter.UsesAbsolute,
		Domain:              filter.Domain,
		SourceIP:            filter.SourceIP,
		SortOrgURL:          filter.sortLink(report.SortByOrgName),
		SortOrgIndicator:    filter.sortIndicator(report.SortByOrgName),
		SortDomainURL:       filter.sortLink(report.SortByDomain),
		SortDomainIndicator: filter.sortIndicator(report.SortByDomain),
		SortPeriodURL:       filter.sortLink(report.SortByDateBegin),
		SortPeriodIndicator: filter.sortIndicator(report.SortByDateBegin),
		ExportURL:           "/export/berichte.csv?" + filter.values().Encode(),
		Rows: reportsRowsData{
			Rows:        reportRows(page.Reports),
			HasMore:     page.NextCursor != "",
			NextPageURL: reportsPageURL(filter, page.NextCursor),
		},
	}

	if filter.UsesAbsolute {
		data.PeriodFromLabel = dayISO(filter.Period.Begin)
		data.PeriodToLabel = dayISO(filter.Period.End)
	} else {
		fp := filterParams{Days: filter.PeriodDays}
		data.PeriodOptions = fp.options()
	}

	dispositions := []report.Disposition{"", report.DispositionNone, report.DispositionQuarantine, report.DispositionReject, report.DispositionUnknown}
	data.DispositionOptions = make([]dispositionOptionView, len(dispositions))
	for i, d := range dispositions {
		label := "Alle"
		if d != "" {
			label = dispositionLabel(d)
		}
		data.DispositionOptions[i] = dispositionOptionView{Value: string(d), Label: label, Selected: d == filter.Disposition}
	}

	groups := []report.GroupBy{report.GroupByNone, report.GroupByDomain, report.GroupByOrg}
	groupLabels := map[report.GroupBy]string{
		report.GroupByNone:   "Keine",
		report.GroupByDomain: "Domain",
		report.GroupByOrg:    "Organisation",
	}
	data.GroupOptions = make([]groupOptionView, len(groups))
	for i, g := range groups {
		data.GroupOptions[i] = groupOptionView{Value: string(g), Label: groupLabels[g], Selected: g == filter.GroupBy}
	}

	return data
}

// reportsPageURL baut den hx-get-Link für "Weitere laden" — derselbe
// Filter, ergänzt um den Cursor der nächsten Seite.
func reportsPageURL(filter reportsFilter, cursor string) string {
	v := filter.values()
	v.Set("cursor", cursor)
	return "/berichte/seite?" + v.Encode()
}

// --- Handler --------------------------------------------------------------

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	filter, err := parseReportsFilter(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.Reports.List(r.Context(), filter.query("", reportsPageSize))
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := buildReportsPageData(filter, page)
	data.Nav = navItems(r.URL.Path)
	if err := s.views.render(w, r, "reports.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleReportsPage liefert die nächste Seite als HTML-Fragment (nur die
// zusätzlichen Tabellenzeilen plus neue "Weitere laden"-Zeile) — für den
// htmx-Aufruf des Ladeknopfs, kein voller Seitenaufbau
// (MIGRATIONSPLAN.md Abschnitt 7).
func (s *Server) handleReportsPage(w http.ResponseWriter, r *http.Request) {
	filter, err := parseReportsFilter(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	cursor := r.URL.Query().Get("cursor")

	page, err := s.deps.Reports.List(r.Context(), filter.query(cursor, reportsPageSize))
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := reportsRowsData{
		Rows:        reportRows(page.Reports),
		HasMore:     page.NextCursor != "",
		NextPageURL: reportsPageURL(filter, page.NextCursor),
	}
	if err := s.views.renderNamed(w, r, "reports.html", "rows", data); err != nil {
		s.serverError(w, r, err)
	}
}

// --- Detailansicht ---------------------------------------------------------

type reportDetailPageData struct {
	Title string
	Nav   []navItem

	OrgName          string
	Email            string
	ExtraContactInfo string
	ReportID         string
	RangeLabel       string

	Domain          string
	Policy          string
	SubdomainPolicy string
	DKIMAlignment   string
	SPFAlignment    string
	Percentage      int

	Records []recordRowView
}

type recordRowView struct {
	SourceIP    string
	Count       int
	Disposition string
	DKIM        string
	SPF         string
}

func buildReportDetailData(full *report.AggregateReport) reportDetailPageData {
	records := make([]recordRowView, len(full.Records))
	for i, rec := range full.Records {
		records[i] = recordRowView{
			SourceIP:    rec.SourceIP.String(),
			Count:       rec.Count,
			Disposition: string(rec.Evaluated.Disposition),
			DKIM:        string(rec.Evaluated.DKIM),
			SPF:         string(rec.Evaluated.SPF),
		}
	}

	return reportDetailPageData{
		Title:            fmt.Sprintf("Bericht %s", full.Metadata.ReportID),
		OrgName:          full.Metadata.OrgName,
		Email:            full.Metadata.Email,
		ExtraContactInfo: full.Metadata.ExtraContactInfo,
		ReportID:         full.Metadata.ReportID,
		RangeLabel: full.Metadata.Range.Begin.Format("2006-01-02 15:04") + " bis " +
			full.Metadata.Range.End.Format("2006-01-02 15:04"),
		Domain:          full.Policy.Domain.String(),
		Policy:          string(full.Policy.Policy),
		SubdomainPolicy: string(full.Policy.SubdomainPolicy),
		DKIMAlignment:   string(full.Policy.DKIMAlignment),
		SPFAlignment:    string(full.Policy.SPFAlignment),
		Percentage:      full.Policy.Percentage,
		Records:         records,
	}
}

func (s *Server) handleReportDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	full, err := s.deps.Reports.Get(r.Context(), report.ReportID(id))
	if err != nil {
		// Kein eigener 404-Pfad: report.Repository.FindByID meldet
		// "nicht gefunden" über einen technischen Fehler (sql.ErrNoRows,
		// siehe internal/infra/sqlite), keinen Domänen-Sentinel — dieselbe
		// Behandlung wie zuvor internal/ui/reports.View.showDetail (jeder
		// Fehler führt zur selben generischen Fehleranzeige, unabhängig
		// von der Ursache).
		s.serverError(w, r, err)
		return
	}

	data := buildReportDetailData(full)
	data.Nav = navItems("/berichte")
	if err := s.views.render(w, r, "report_detail.html", data); err != nil {
		s.serverError(w, r, err)
	}
}
