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

// reportsPageSize is the page size requested per load step — the same
// lazy-loading logic as formerly internal/ui/reports.View (pageSize),
// here via htmx instead of a Fyne button (section 7: "GET
// /berichte/seite | next page (htmx, keyset cursor)").
const reportsPageSize = 50

// reportsFilter combines all filter/sort/grouping parameters of
// /berichte — read from the URL: either the shared filter bar
// (zeitraum/domain, like the overview) or the absolute drill-down
// parameters of a chart click (von/bis/quelle/disposition,
// MIGRATIONSPLAN.md section 9.6).
type reportsFilter struct {
	Period       report.DateRange
	UsesAbsolute bool
	PeriodDays   int // only valid when !UsesAbsolute

	Domain      string
	SourceIP    string
	Disposition report.Disposition // "" means: no filter

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

// parseGroupBy/parseSortField/parseSortDirection accept only exactly the
// values report.Query supports (whose constant strings happen to match
// the query parameter values used here, see
// report.GroupBy/SortField/SortDirection) — any other value (including a
// manipulated link) falls back to the respective default instead of
// rejecting the request.
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

// query builds the repository query for this filter with the given
// keyset cursor (empty for the first page) and the given page size — for
// the reports table itself always reportsPageSize, for the CSV export a
// larger page size (handlers_export.go: fewer database round trips,
// without loading the entire filtered set at once).
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

// values builds the query parameters that fully describe the current
// filter — the basis for "load more" (with an added cursor) and the
// sortable column headers (with a changed sortierung/sortrichtung), so
// neither of the two loses the rest of the filter.
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

// sortLink builds the link for a sortable column header: sorts by field,
// in reverse direction if field is already the active sort, otherwise
// ascending.
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

// --- Template data ---------------------------------------------------------

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

	// ExportURL exports the ENTIRE currently filtered set as CSV (not just
	// the loaded page, see handlers_export.go) — the same filter the
	// table is currently showing.
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

// reportsPageURL builds the hx-get link for "load more" — the same
// filter, with the next page's cursor added.
func reportsPageURL(filter reportsFilter, cursor string) string {
	v := filter.values()
	v.Set("cursor", cursor)
	return "/berichte/seite?" + v.Encode()
}

// --- Handlers ---------------------------------------------------------------

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

// handleReportsPage returns the next page as an HTML fragment (only the
// additional table rows plus a new "load more" row) — for the htmx call
// from the load-more button, not a full page build (MIGRATIONSPLAN.md
// section 7).
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

// --- Detail view -------------------------------------------------------------

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

	// OnlyFailed/ToggleURL/ToggleLabel carry the "show only failed"
	// toggle (?nur_fehler=1) — a pure GET display filter on the already
	// loaded records of this one report, not a state change, hence a
	// plain link instead of a POST form (like "reset filter" on the list
	// views).
	OnlyFailed       bool
	TotalRecordCount int
	ShownRecordCount int
	ToggleURL        string
	ToggleLabel      string

	Records []recordRowView
}

type recordRowView struct {
	SourceIP        string
	Count           int
	Disposition     string
	DispositionTone string
	DKIM            string
	DKIMTone        string
	SPF             string
	SPFTone         string
	Detail          recordDetailView
}

// buildReportDetailData builds the display data for the report detail
// page. onlyFailed hides records where DMARC passed
// (rec.Evaluated.PassesDMARC()) — "failed" is defined uniformly here as
// !PassesDMARC(), just like in the failures view
// (handlers_failedrecords.go).
func buildReportDetailData(full *report.AggregateReport, onlyFailed bool) reportDetailPageData {
	shown := full.Records
	if onlyFailed {
		shown = make([]report.Record, 0, len(full.Records))
		for _, rec := range full.Records {
			if !rec.Evaluated.PassesDMARC() {
				shown = append(shown, rec)
			}
		}
	}

	records := make([]recordRowView, len(shown))
	for i, rec := range shown {
		records[i] = recordRowView{
			SourceIP:        rec.SourceIP.String(),
			Count:           rec.Count,
			Disposition:     string(rec.Evaluated.Disposition),
			DispositionTone: dispositionTone(rec.Evaluated.Disposition),
			DKIM:            string(rec.Evaluated.DKIM),
			DKIMTone:        authResultTone(rec.Evaluated.DKIM),
			SPF:             string(rec.Evaluated.SPF),
			SPFTone:         authResultTone(rec.Evaluated.SPF),
			Detail:          buildRecordDetailView(rec.Identifiers, rec.Auth, rec.Evaluated.Reasons),
		}
	}

	basePath := fmt.Sprintf("/berichte/%d", full.ID)
	toggleURL := basePath + "?nur_fehler=1"
	toggleLabel := "Nur fehlgeschlagene anzeigen"
	if onlyFailed {
		toggleURL = basePath
		toggleLabel = "Alle anzeigen"
	}

	return reportDetailPageData{
		Title:            fmt.Sprintf("Bericht %s", full.Metadata.ReportID),
		OrgName:          full.Metadata.OrgName,
		Email:            full.Metadata.Email,
		ExtraContactInfo: full.Metadata.ExtraContactInfo,
		ReportID:         full.Metadata.ReportID,
		RangeLabel: full.Metadata.Range.Begin.Format("2006-01-02 15:04") + " bis " +
			full.Metadata.Range.End.Format("2006-01-02 15:04"),
		Domain:           full.Policy.Domain.String(),
		Policy:           string(full.Policy.Policy),
		SubdomainPolicy:  string(full.Policy.SubdomainPolicy),
		DKIMAlignment:    string(full.Policy.DKIMAlignment),
		SPFAlignment:     string(full.Policy.SPFAlignment),
		Percentage:       full.Policy.Percentage,
		OnlyFailed:       onlyFailed,
		TotalRecordCount: len(full.Records),
		ShownRecordCount: len(shown),
		ToggleURL:        toggleURL,
		ToggleLabel:      toggleLabel,
		Records:          records,
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
		// No dedicated 404 path: report.Repository.FindByID reports "not
		// found" via a technical error (sql.ErrNoRows, see
		// internal/infra/sqlite), not a domain sentinel — the same
		// handling as formerly internal/ui/reports.View.showDetail (every
		// error leads to the same generic error display, regardless of
		// cause).
		s.serverError(w, r, err)
		return
	}

	onlyFailed := r.URL.Query().Get("nur_fehler") == "1"
	data := buildReportDetailData(full, onlyFailed)
	data.Nav = navItems("/berichte")
	if err := s.views.render(w, r, "report_detail.html", data); err != nil {
		s.serverError(w, r, err)
	}
}
