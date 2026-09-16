// Package sourceinfo implementiert domain/sources.Enricher: rDNS-/
// PTR-Auflösung von Quell-IPs mit Cache (FEATURES.md Vorschlag 11.2) und
// Erkennung bekannter Diensteanbieter über den PTR-Hostnamen (Vorschlag
// 11.3).
//
// Bewusst nur hostnamenbasiert, nicht zusätzlich über IP-Bereiche: die
// veröffentlichten IP-Bereiche der großen Anbieter (Google, Microsoft,
// Mailchimp, SendGrid, …) ändern sich fortlaufend, und ohne einen Prozess,
// der sie aktuell hält, wäre eine hier fest einprogrammierte CIDR-Liste
// nach kurzer Zeit stille falsche Auskunft — schlechter als gar keine.
// PTR-Namen sind stabil genug, um allein zu tragen.
package sourceinfo

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// lookupTimeout begrenzt eine einzelne PTR-Auflösung — eine hängende
// DNS-Anfrage darf die Sendequellen-Ansicht nicht blockieren.
const lookupTimeout = 3 * time.Second

// addrResolver ist der Ausschnitt von *net.Resolver, den Enricher braucht
// — als Schnittstelle, damit Tests ohne echte DNS-Auflösung laufen.
type addrResolver interface {
	LookupAddr(ctx context.Context, addr string) ([]string, error)
}

// Enricher implementiert sources.Enricher. Ergebnisse werden pro
// Prozesslaufzeit unbegrenzt zwischengespeichert (kein TTL/Eviction) —
// PTR-Einträge ändern sich in der Praxis selten, und die Anwendung läuft
// nicht dauerhaft im Hintergrund, sodass veraltete Cache-Einträge kein
// relevantes Risiko sind.
type Enricher struct {
	resolver addrResolver
	cache    sync.Map // map[string]sources.Enrichment, Key: SourceIP.String()
}

var _ sources.Enricher = (*Enricher)(nil)

// NewEnricher erzeugt einen einsatzbereiten Enricher gegen den
// System-Resolver.
func NewEnricher() *Enricher {
	return &Enricher{resolver: net.DefaultResolver}
}

// Enrich löst ip per PTR auf und gleicht den Hostnamen gegen die Liste
// bekannter Dienste ab. Liefert nie einen Fehler — eine nicht auflösbare
// PTR oder ein nicht erkannter Dienst sind normale Ausgänge (siehe
// domain/sources.Enricher-Dokumentation).
func (e *Enricher) Enrich(ctx context.Context, ip report.SourceIP) sources.Enrichment {
	key := ip.String()
	if cached, ok := e.cache.Load(key); ok {
		return cached.(sources.Enrichment)
	}

	enrichment := e.resolve(ctx, key)
	e.cache.Store(key, enrichment)
	return enrichment
}

func (e *Enricher) resolve(ctx context.Context, ip string) sources.Enrichment {
	lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	names, err := e.resolver.LookupAddr(lookupCtx, ip)
	if err != nil || len(names) == 0 {
		return sources.Enrichment{}
	}

	hostname := strings.TrimSuffix(names[0], ".")
	return sources.Enrichment{Hostname: hostname, Service: detectService(hostname)}
}

// serviceRule bildet Hostnamen-Suffixe auf einen erkannten Diensteanbieter
// ab (FEATURES.md Vorschlag 11.3: "kuratierte Liste ... Abgleich über
// PTR"). Suffixe sind kleingeschrieben, Vergleich case-insensitiv.
type serviceRule struct {
	name     string
	suffixes []string
}

// knownServices ist eine kuratierte, nicht erschöpfende Auswahl gängiger
// E-Mail-Diensteanbieter — ergänzbar, ohne den Rest des Pakets zu ändern.
var knownServices = []serviceRule{
	{name: "Google Workspace", suffixes: []string{".google.com", ".googlemail.com"}},
	{name: "Microsoft 365", suffixes: []string{".outlook.com", ".protection.outlook.com"}},
	{name: "Mailchimp", suffixes: []string{".mailchimp.com", ".mcsv.net", ".mcdlv.net"}},
	{name: "SendGrid", suffixes: []string{".sendgrid.net"}},
	{name: "Brevo (vormals Sendinblue)", suffixes: []string{".sendinblue.com", ".brevo.com"}},
	{name: "Postmark", suffixes: []string{".mtasv.net"}},
}

func detectService(hostname string) string {
	if hostname == "" {
		return ""
	}
	h := strings.ToLower(hostname)
	for _, rule := range knownServices {
		for _, suffix := range rule.suffixes {
			if strings.HasSuffix(h, suffix) {
				return rule.name
			}
		}
	}
	return ""
}
