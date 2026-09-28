// Package sourceinfo implements domain/sources.Enricher: rDNS/PTR
// resolution of source IPs with a cache (FEATURES.md proposal 11.2) and
// detection of known service providers via the PTR hostname (proposal
// 11.3).
//
// Deliberately hostname-based only, not additionally via IP ranges: the
// published IP ranges of major providers (Google, Microsoft, Mailchimp,
// SendGrid, …) change continuously, and without a process that keeps them
// current, a CIDR list hardcoded here would quickly become silently wrong
// information — worse than none at all. PTR names are stable enough to
// carry this on their own.
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

// lookupTimeout limits a single PTR resolution — a hanging DNS request
// must not block the traffic sources view.
const lookupTimeout = 3 * time.Second

// addrResolver is the slice of *net.Resolver that Enricher needs — as an
// interface, so tests can run without real DNS resolution.
type addrResolver interface {
	LookupAddr(ctx context.Context, addr string) ([]string, error)
}

// Enricher implements sources.Enricher. Results are cached without limit
// for the process's lifetime (no TTL/eviction) — PTR entries rarely change
// in practice, and the application doesn't run in the background
// indefinitely, so stale cache entries pose no relevant risk.
type Enricher struct {
	resolver addrResolver
	cache    sync.Map // map[string]sources.Enrichment, key: SourceIP.String()
}

var _ sources.Enricher = (*Enricher)(nil)

// NewEnricher creates a ready-to-use Enricher against the system resolver.
func NewEnricher() *Enricher {
	return &Enricher{resolver: net.DefaultResolver}
}

// Enrich resolves ip via PTR and matches the hostname against the list of
// known services. Never returns an error — an unresolvable PTR or an
// unrecognized service are normal outcomes (see the domain/sources.Enricher
// documentation).
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

// serviceRule maps hostname suffixes to a recognized service provider
// (FEATURES.md proposal 11.3: "curated list ... matched via PTR"). Suffixes
// are lowercase, comparison is case-insensitive.
type serviceRule struct {
	name     string
	suffixes []string
}

// knownServices is a curated, non-exhaustive selection of common email
// service providers — extendable without changing the rest of the package.
var knownServices = []serviceRule{
	{name: "Google Workspace", suffixes: []string{".google.com", ".googlemail.com"}},
	{name: "Microsoft 365", suffixes: []string{".outlook.com", ".protection.outlook.com"}},
	{name: "Mailchimp", suffixes: []string{".mailchimp.com", ".mcsv.net", ".mcdlv.net"}},
	{name: "SendGrid", suffixes: []string{".sendgrid.net"}},
	{name: "Brevo (formerly Sendinblue)", suffixes: []string{".sendinblue.com", ".brevo.com"}},
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
