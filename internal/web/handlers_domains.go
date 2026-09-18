package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

// domainsPageSize ist die je Ladeschritt angeforderte Seitengröße —
// dieselbe Lazy-Nachladelogik wie /berichte und /quellen.
const domainsPageSize = 50

// domainsFilter fasst Filter und Sortierung von /domains zusammen.
// Genau wie sourcesFilter kennt domainstats.Query keine
// Drill-down-spezifischen Felder — nur Zeitraum, Domain (geteilte
// Filterleiste) und ein Sortierfeld.
type domainsFilter struct {
	Period    filterParams
	SortField domainstats.SortField
}

func parseDomainsFilter(r *http.Request) domainsFilter {
	fp := parseFilterParams(r)
	return domainsFilter{
		Period:    fp,
		SortField: parseDomainsSortField(r.URL.Query().Get("sortierung")),
	}
}

func parseDomainsSortField(raw string) domainstats.SortField {
	if domainstats.SortField(raw) == domainstats.SortByDomain {
		return domainstats.SortByDomain
	}
	return domainstats.SortByVolume
}

func (f domainsFilter) query(cursor string, limit int) (domainstats.Query, error) {
	q, err := f.Period.query()
	if err != nil {
		return domainstats.Query{}, err
	}
	return domainstats.Query{
		Period:    &q.Period,
		Domain:    f.Period.Domain,
		SortField: f.SortField,
		Limit:     limit,
		Cursor:    cursor,
	}, nil
}

func (f domainsFilter) values() url.Values {
	v := url.Values{}
	v.Set("zeitraum", strconv.Itoa(f.Period.Days))
	if f.Period.Domain != "" {
		v.Set("domain", f.Period.Domain)
	}
	v.Set("sortierung", string(f.SortField))
	return v
}

func (f domainsFilter) sortLink(field domainstats.SortField) string {
	next := f
	next.SortField = field
	return "/domains?" + next.values().Encode()
}

func domainsPageURL(filter domainsFilter, cursor string) string {
	v := filter.values()
	v.Set("cursor", cursor)
	return "/domains/seite?" + v.Encode()
}

// domainReportsURL verlinkt eine Domain-Zeile auf die Berichte-Liste,
// gefiltert auf genau diese Domain und denselben Zeitraum — der
// eigentliche Zweck dieser Ansicht: eine Zeile pro Domain zum Überblicken,
// mit einem Klick weiter zu den einzelnen Reports dahinter (die die
// Domains-Ansicht bewusst NICHT ersetzt, siehe domainstats.Stat.ReportCount).
func domainReportsURL(days int, domain string) string {
	v := url.Values{}
	v.Set("zeitraum", strconv.Itoa(days))
	v.Set("domain", domain)
	return "/berichte?" + v.Encode()
}

// --- Vorlagendaten -------------------------------------------------------

type domainRowView struct {
	Domain          string
	Total           int
	PassRate        string
	ReportCount     int
	DistinctSources int
	ReportsURL      string
}

func domainRows(days int, stats []domainstats.Stat) []domainRowView {
	rows := make([]domainRowView, len(stats))
	for i, s := range stats {
		rows[i] = domainRowView{
			Domain:          s.Domain.String(),
			Total:           s.TotalCount,
			PassRate:        formatPercent(s.PassRate),
			ReportCount:     s.ReportCount,
			DistinctSources: s.DistinctSources,
			ReportsURL:      domainReportsURL(days, s.Domain.String()),
		}
	}
	return rows
}

type domainsRowsData struct {
	Rows        []domainRowView
	HasMore     bool
	NextPageURL string
}

type domainsPageData struct {
	Title string
	Nav   []navItem

	Domain        string
	PeriodOptions []periodOptionView

	SortVolumeURL string
	SortDomainURL string
	SortField     string

	// ExportURL: siehe reportsPageData.ExportURL.
	ExportURL string

	Rows domainsRowsData
}

func buildDomainsPageData(filter domainsFilter, page domainstats.Page) domainsPageData {
	return domainsPageData{
		Title:         "Domains",
		Domain:        filter.Period.Domain,
		PeriodOptions: filter.Period.options(),
		SortVolumeURL: filter.sortLink(domainstats.SortByVolume),
		SortDomainURL: filter.sortLink(domainstats.SortByDomain),
		SortField:     string(filter.SortField),
		ExportURL:     "/export/domains.csv?" + filter.values().Encode(),
		Rows: domainsRowsData{
			Rows:        domainRows(filter.Period.Days, page.Stats),
			HasMore:     page.NextCursor != "",
			NextPageURL: domainsPageURL(filter, page.NextCursor),
		},
	}
}

// --- Handler --------------------------------------------------------------

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	filter := parseDomainsFilter(r)
	q, err := filter.query("", domainsPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.Domains.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := buildDomainsPageData(filter, page)
	data.Nav = navItems(r.URL.Path)
	if err := s.views.render(w, r, "domains.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) handleDomainsPage(w http.ResponseWriter, r *http.Request) {
	filter := parseDomainsFilter(r)
	q, err := filter.query(r.URL.Query().Get("cursor"), domainsPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.Domains.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := domainsRowsData{
		Rows:        domainRows(filter.Period.Days, page.Stats),
		HasMore:     page.NextCursor != "",
		NextPageURL: domainsPageURL(filter, page.NextCursor),
	}
	if err := s.views.renderNamed(w, r, "domains.html", "rows", data); err != nil {
		s.serverError(w, r, err)
	}
}
