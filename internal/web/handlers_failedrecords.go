package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

// failedRecordsPageSize ist die je Ladeschritt angeforderte Seitengröße —
// dieselbe Lazy-Nachladelogik wie /berichte/domains/quellen.
const failedRecordsPageSize = 50

// failedRecordsFilter fasst Filter und Sortierung von /fehlschlaege
// zusammen: die geteilte Filterleiste (Zeitraum/Domain) plus Quell-IP
// (wie /berichte) und ein Sortierfeld.
type failedRecordsFilter struct {
	Period    filterParams
	SourceIP  string
	SortField failedrecords.SortField
}

func parseFailedRecordsFilter(r *http.Request) failedRecordsFilter {
	fp := parseFilterParams(r)
	return failedRecordsFilter{
		Period:    fp,
		SourceIP:  strings.TrimSpace(r.URL.Query().Get("quelle")),
		SortField: parseFailedRecordsSortField(r.URL.Query().Get("sortierung")),
	}
}

func parseFailedRecordsSortField(raw string) failedrecords.SortField {
	if failedrecords.SortField(raw) == failedrecords.SortBySourceIP {
		return failedrecords.SortBySourceIP
	}
	return failedrecords.SortByDate
}

func (f failedRecordsFilter) query(cursor string, limit int) (failedrecords.Query, error) {
	q, err := f.Period.query()
	if err != nil {
		return failedrecords.Query{}, err
	}
	return failedrecords.Query{
		Period:    &q.Period,
		Domain:    f.Period.Domain,
		SourceIP:  f.SourceIP,
		SortField: f.SortField,
		Limit:     limit,
		Cursor:    cursor,
	}, nil
}

func (f failedRecordsFilter) values() url.Values {
	v := url.Values{}
	v.Set("zeitraum", strconv.Itoa(f.Period.Days))
	if f.Period.Domain != "" {
		v.Set("domain", f.Period.Domain)
	}
	if f.SourceIP != "" {
		v.Set("quelle", f.SourceIP)
	}
	v.Set("sortierung", string(f.SortField))
	return v
}

// sortLink baut den Link für einen sortierbaren Spaltenkopf — anders als
// reportsFilter.sortLink ohne Richtungsumkehr, es gibt hier nur genau
// eine sinnvolle Richtung je Sortierfeld (neuester Bericht zuerst bzw.
// Quell-IP aufsteigend).
func (f failedRecordsFilter) sortLink(field failedrecords.SortField) string {
	next := f
	next.SortField = field
	return "/fehlschlaege?" + next.values().Encode()
}

func failedRecordsPageURL(filter failedRecordsFilter, cursor string) string {
	v := filter.values()
	v.Set("cursor", cursor)
	return "/fehlschlaege/seite?" + v.Encode()
}

// --- Vorlagendaten -------------------------------------------------------

type failedRecordRowView struct {
	PeriodLabel string
	ReportURL   string
	OrgName     string
	Domain      string

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

func failedRecordRows(records []failedrecords.Record) []failedRecordRowView {
	rows := make([]failedRecordRowView, len(records))
	for i, rec := range records {
		rows[i] = failedRecordRowView{
			PeriodLabel:     rec.PeriodBegin.Format("2006-01-02") + " – " + rec.PeriodEnd.Format("2006-01-02"),
			ReportURL:       "/berichte/" + strconv.FormatInt(int64(rec.ReportID), 10),
			OrgName:         rec.OrgName,
			Domain:          rec.PolicyDomain.String(),
			SourceIP:        rec.SourceIP.String(),
			Count:           rec.Count,
			Disposition:     string(rec.Disposition),
			DispositionTone: dispositionTone(rec.Disposition),
			DKIM:            string(rec.DKIM),
			DKIMTone:        authResultTone(rec.DKIM),
			SPF:             string(rec.SPF),
			SPFTone:         authResultTone(rec.SPF),
			Detail:          buildRecordDetailView(rec.Identifiers, rec.Auth, rec.Reasons),
		}
	}
	return rows
}

type failedRecordsRowsData struct {
	Rows        []failedRecordRowView
	HasMore     bool
	NextPageURL string
}

type failedRecordsPageData struct {
	Title string
	Nav   []navItem

	Domain        string
	SourceIP      string
	PeriodOptions []periodOptionView

	SortDateURL     string
	SortSourceIPURL string

	// ExportURL: siehe reportsPageData.ExportURL.
	ExportURL string

	Rows failedRecordsRowsData
}

func buildFailedRecordsPageData(filter failedRecordsFilter, page failedrecords.Page) failedRecordsPageData {
	return failedRecordsPageData{
		Title:           "Fehlschläge",
		Domain:          filter.Period.Domain,
		SourceIP:        filter.SourceIP,
		PeriodOptions:   filter.Period.options(),
		SortDateURL:     filter.sortLink(failedrecords.SortByDate),
		SortSourceIPURL: filter.sortLink(failedrecords.SortBySourceIP),
		ExportURL:       "/export/fehlschlaege.csv?" + filter.values().Encode(),
		Rows: failedRecordsRowsData{
			Rows:        failedRecordRows(page.Records),
			HasMore:     page.NextCursor != "",
			NextPageURL: failedRecordsPageURL(filter, page.NextCursor),
		},
	}
}

// --- Handler --------------------------------------------------------------

func (s *Server) handleFailedRecords(w http.ResponseWriter, r *http.Request) {
	filter := parseFailedRecordsFilter(r)
	q, err := filter.query("", failedRecordsPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.FailedRecords.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := buildFailedRecordsPageData(filter, page)
	data.Nav = navItems(r.URL.Path)
	if err := s.views.render(w, r, "failed_records.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleFailedRecordsPage liefert die nächste Seite als HTML-Fragment —
// für den htmx-Aufruf des Ladeknopfs, kein voller Seitenaufbau (siehe
// handleDomainsPage).
func (s *Server) handleFailedRecordsPage(w http.ResponseWriter, r *http.Request) {
	filter := parseFailedRecordsFilter(r)
	q, err := filter.query(r.URL.Query().Get("cursor"), failedRecordsPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.FailedRecords.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := failedRecordsRowsData{
		Rows:        failedRecordRows(page.Records),
		HasMore:     page.NextCursor != "",
		NextPageURL: failedRecordsPageURL(filter, page.NextCursor),
	}
	if err := s.views.renderNamed(w, r, "failed_records.html", "rows", data); err != nil {
		s.serverError(w, r, err)
	}
}
