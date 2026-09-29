package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// sourcesPageSize is the page size requested per load step — the same
// lazy-loading logic as /berichte (reportsPageSize) and formerly
// internal/ui/sources.View.
const sourcesPageSize = 50

// sourcesFilter combines the filter and sort order of /quellen. Unlike
// /berichte, domainsources.Query has no drill-down-specific fields
// (source IP/disposition would be meaningless here too: this view IS
// already aggregated by source IP) — only period, domain (shared filter
// bar) and a sort field.
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

// --- Template data ---------------------------------------------------------

type sourceRowView struct {
	IP       string
	Total    int
	PassRate string
	Hostname string
	Service  string

	// EinordnungLabel and EinordnungTon explain how to interpret this
	// source (see classifySource) — the same reasoning that would
	// otherwise only happen in the viewer's head: DKIM without SPF is
	// usually harmless forwarding, whereas a DKIM failure is a reason to
	// look closer.
	EinordnungLabel string
	EinordnungTon   string
	// GleicherHosterWieIMAP is set when this source's PTR hostname shares
	// the same (coarse) operator domain part as the configured IMAP
	// account (see registrableDomain) — another signal for "our own
	// infrastructure", in addition to the DKIM/SPF check.
	GleicherHosterWieIMAP bool
}

const sourcesUnknownValue = "—"

// classifySource classifies a sending source based on its separate
// DKIM/SPF pass rates. The thresholds (0.9/0.5) are rough rules of thumb
// for "essentially passes/doesn't pass" across many messages, not an
// attempt at an exact statistical boundary.
//
//   - DKIM AND SPF pass mostly: clearly authorized.
//   - DKIM passes, SPF doesn't: DMARC still passes (dkim OR spf is
//     enough), but the pattern is typical of mail forwarding — the DKIM
//     signature survives the forward, SPF almost always breaks because
//     the forwarding IP isn't in the original domain's SPF record.
//   - DKIM predominantly does NOT pass: a genuine spoof couldn't pass
//     DKIM (it lacks the domain's private key for that) — worth a
//     closer look.
//   - everything in between: ambiguous, also worth a look.
func classifySource(s domainsources.Stat) (label, tone string) {
	switch {
	case s.DKIMPassRate >= 0.9 && s.SPFPassRate >= 0.9:
		return "Autorisiert (DKIM & SPF)", "good"
	case s.DKIMPassRate >= 0.9:
		return "Autorisiert (vermutlich Weiterleitung)", "good"
	case s.DKIMPassRate < 0.5:
		return "Nicht bestätigt — prüfen", "critical"
	default:
		return "Teilweise bestätigt — prüfen", "warning"
	}
}

// registrableDomain returns a rough approximation of a hostname's
// operator domain part: the last two "."-separated labels (e.g.
// "kasserver.com" from "dd33832.kasserver.com"). Not a general
// public-suffix parser (that would need a maintained list for multi-part
// TLDs like ".co.uk") — good enough for the coarse comparison used here:
// "does this run through the same host as the IMAP account?".
func registrableDomain(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func sourceRows(stats []domainsources.Stat, imapHost string) []sourceRowView {
	imapDomain := registrableDomain(imapHost)
	rows := make([]sourceRowView, len(stats))
	for i, s := range stats {
		hostname := s.Enrichment.Hostname
		sameHoster := hostname != "" && imapDomain != "" && registrableDomain(hostname) == imapDomain
		if hostname == "" {
			hostname = sourcesUnknownValue
		}
		service := s.Enrichment.Service
		if service == "" {
			service = sourcesUnknownValue
		}
		label, tone := classifySource(s)
		rows[i] = sourceRowView{
			IP:                    s.SourceIP.String(),
			Total:                 s.TotalCount,
			PassRate:              formatPercent(s.PassRate),
			Hostname:              hostname,
			Service:               service,
			EinordnungLabel:       label,
			EinordnungTon:         tone,
			GleicherHosterWieIMAP: sameHoster,
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

	// ExportURL: see reportsPageData.ExportURL.
	ExportURL string

	Rows sourcesRowsData
}

func buildSourcesPageData(filter sourcesFilter, page domainsources.Page, imapHost string) sourcesPageData {
	return sourcesPageData{
		Title:         "Sendequellen",
		Domain:        filter.Period.Domain,
		PeriodOptions: filter.Period.options(),
		SortVolumeURL: filter.sortLink(domainsources.SortByVolume),
		SortIPURL:     filter.sortLink(domainsources.SortByIP),
		SortField:     string(filter.SortField),
		ExportURL:     "/export/quellen.csv?" + filter.values().Encode(),
		Rows: sourcesRowsData{
			Rows:        sourceRows(page.Stats, imapHost),
			HasMore:     page.NextCursor != "",
			NextPageURL: sourcesPageURL(filter, page.NextCursor),
		},
	}
}

// --- Handlers ---------------------------------------------------------------

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

	data := buildSourcesPageData(filter, page, s.deps.IMAPHost)
	data.Nav = navItems(r.URL.Path)
	if err := s.views.render(w, r, "sources.html", data); err != nil {
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
		Rows:        sourceRows(page.Stats, s.deps.IMAPHost),
		HasMore:     page.NextCursor != "",
		NextPageURL: sourcesPageURL(filter, page.NextCursor),
	}
	if err := s.views.renderNamed(w, r, "sources.html", "rows", data); err != nil {
		s.serverError(w, r, err)
	}
}
