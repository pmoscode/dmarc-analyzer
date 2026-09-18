package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

	// EinordnungLabel und EinordnungTon erklären, wie diese Quelle zu
	// deuten ist (siehe classifySource) — dieselbe Erklärung, die sonst
	// nur im Kopf des Betrachters stattfindet: DKIM ohne SPF ist meist
	// eine harmlose Weiterleitung, DKIM-Fehlschlag dagegen ein Grund zum
	// genaueren Hinsehen.
	EinordnungLabel string
	EinordnungTon   string
	// GleicherHosterWieIMAP ist gesetzt, wenn der PTR-Hostname dieser
	// Quelle denselben (groben) Betreiber-Domainteil trägt wie das
	// konfigurierte IMAP-Konto (siehe registrableDomain) — ein weiteres
	// Indiz für "eigene Infrastruktur", zusätzlich zur DKIM/SPF-Prüfung.
	GleicherHosterWieIMAP bool
}

const sourcesUnknownValue = "—"

// classifySource ordnet eine Sendequelle anhand ihrer getrennten DKIM-/
// SPF-Bestehensrate ein. Die Schwellenwerte (0.9/0.5) sind grobe
// Faustregeln für "im Wesentlichen besteht/besteht nicht" über viele
// Nachrichten hinweg, kein Versuch einer exakten statistischen Grenze.
//
//   - DKIM UND SPF bestehen weitgehend: eindeutig autorisiert.
//   - DKIM besteht, SPF nicht: DMARC besteht trotzdem (dkim ODER spf
//     reicht), das Muster ist aber typisch für Mail-Weiterleitung — die
//     DKIM-Signatur übersteht die Weiterleitung, SPF bricht fast immer,
//     weil die weiterleitende IP nicht im SPF-Record der ursprünglichen
//     Domain steht.
//   - DKIM besteht überwiegend NICHT: eine echte Fälschung könnte DKIM
//     nicht bestehen (dafür fehlt der private Schlüssel der Domain) —
//     das lohnt einen genaueren Blick.
//   - alles dazwischen: uneindeutig, ebenfalls einen Blick wert.
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

// registrableDomain liefert eine grobe Näherung des Betreiber-Domainteils
// eines Hostnamens: die letzten beiden durch "." getrennten Bezeichner
// (z. B. "kasserver.com" aus "dd33832.kasserver.com"). Kein allgemeiner
// Public-Suffix-Parser (der bräuchte eine gepflegte Liste für
// Mehrteil-TLDs wie ".co.uk") — für den hier gebrauchten groben Vergleich
// "läuft das über denselben Hoster wie das IMAP-Konto?" reicht das.
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

	// ExportURL: siehe reportsPageData.ExportURL.
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
