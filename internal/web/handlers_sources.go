package web

import (
	"net/http"
	"net/url"
	"strconv"

	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// sourcesPageSize ist die je Ladeschritt angeforderte Seitengröße —
// dieselbe Lazy-Nachladelogik wie /berichte (reportsPageSize) und zuvor
// internal/ui/sources.View.
const sourcesPageSize = 50

// sourcesFilter fasst Filter und Sortierung von /quellen zusammen.
// Anders als /berichte kennt domainsources.Query keine Drill-down-
// spezifischen Felder (Quell-IP/Disposition wären hier auch fachlich
// sinnlos: diese Ansicht IST bereits nach Quell-IP aggregiert) — nur
// Zeitraum, Domain (geteilte Filterleiste) und ein Sortierfeld.
type sourcesFilter struct {
	Period    filterParams
	SortField domainsources.SortField
}

func parseSourcesFilter(r *http.Request) sourcesFilter {
	fp := parseFilterParams(r)
	return sourcesFilter{
		Period:    fp,
		SortField: parseSourcesSortField(r.URL.Query().Get("sortierung")),
	}
}

func parseSourcesSortField(raw string) domainsources.SortField {
	if domainsources.SortField(raw) == domainsources.SortByIP {
		return domainsources.SortByIP
	}
	return domainsources.SortByVolume
}

func (f sourcesFilter) query(cursor string, limit int) (domainsources.Query, error) {
	q, err := f.Period.query()
	if err != nil {
		return domainsources.Query{}, err
	}
	return domainsources.Query{
		Period:    &q.Period,
		Domain:    f.Period.Domain,
		SortField: f.SortField,
		Limit:     limit,
		Cursor:    cursor,
	}, nil
}

func (f sourcesFilter) values() url.Values {
	v := url.Values{}
	v.Set("zeitraum", strconv.Itoa(f.Period.Days))
	if f.Period.Domain != "" {
		v.Set("domain", f.Period.Domain)
	}
	v.Set("sortierung", string(f.SortField))
	return v
}

func (f sourcesFilter) sortLink(field domainsources.SortField) string {
	next := f
	next.SortField = field
	return "/quellen?" + next.values().Encode()
}

func sourcesPageURL(filter sourcesFilter, cursor string) string {
	v := filter.values()
	v.Set("cursor", cursor)
	return "/quellen/seite?" + v.Encode()
}

// --- Vorlagendaten -------------------------------------------------------

type sourceRowView struct {
	IP       string
	Total    int
	PassRate string
	Hostname string
	Service  string
}

const sourcesUnknownValue = "—"

func sourceRows(stats []domainsources.Stat) []sourceRowView {
	rows := make([]sourceRowView, len(stats))
	for i, s := range stats {
		hostname := s.Enrichment.Hostname
		if hostname == "" {
			hostname = sourcesUnknownValue
		}
		service := s.Enrichment.Service
		if service == "" {
			service = sourcesUnknownValue
		}
		rows[i] = sourceRowView{
			IP:       s.SourceIP.String(),
			Total:    s.TotalCount,
			PassRate: formatPercent(s.PassRate),
			Hostname: hostname,
			Service:  service,
		}
	}
	return rows
}

type sourcesRowsData struct {
	Rows        []sourceRowView
	HasMore     bool
	NextPageURL string
}

type sourcesPageData struct {
	Title string
	Nav   []navItem

	Domain        string
	PeriodOptions []periodOptionView

	SortVolumeURL string
	SortIPURL     string
	SortField     string

	// ExportURL: siehe reportsPageData.ExportURL.
	ExportURL string

	Rows sourcesRowsData
}

func buildSourcesPageData(filter sourcesFilter, page domainsources.Page) sourcesPageData {
	return sourcesPageData{
		Title:         "Sendequellen",
		Domain:        filter.Period.Domain,
		PeriodOptions: filter.Period.options(),
		SortVolumeURL: filter.sortLink(domainsources.SortByVolume),
		SortIPURL:     filter.sortLink(domainsources.SortByIP),
		SortField:     string(filter.SortField),
		ExportURL:     "/export/quellen.csv?" + filter.values().Encode(),
		Rows: sourcesRowsData{
			Rows:        sourceRows(page.Stats),
			HasMore:     page.NextCursor != "",
			NextPageURL: sourcesPageURL(filter, page.NextCursor),
		},
	}
}

// --- Handler --------------------------------------------------------------

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	filter := parseSourcesFilter(r)
	q, err := filter.query("", sourcesPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.Sources.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := buildSourcesPageData(filter, page)
	data.Nav = navItems(r.URL.Path)
	if err := s.views.render(w, "sources.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) handleSourcesPage(w http.ResponseWriter, r *http.Request) {
	filter := parseSourcesFilter(r)
	q, err := filter.query(r.URL.Query().Get("cursor"), sourcesPageSize)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	page, err := s.deps.Sources.List(r.Context(), q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := sourcesRowsData{
		Rows:        sourceRows(page.Stats),
		HasMore:     page.NextCursor != "",
		NextPageURL: sourcesPageURL(filter, page.NextCursor),
	}
	if err := s.views.renderNamed(w, "sources.html", "rows", data); err != nil {
		s.serverError(w, r, err)
	}
}
