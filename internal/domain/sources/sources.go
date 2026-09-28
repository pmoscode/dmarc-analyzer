// Package sources contains the view of sending sources aggregated by
// source IP (IMPLEMENTIERUNG.md section 10.1: "sending sources —
// aggregated by source IP: volume, pass rate, PTR/rDNS, recognized
// service") and the ports for it.
package sources

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Stat is the view of a single source IP, aggregated across all reports
// in the selected period.
type Stat struct {
	SourceIP   report.SourceIP
	TotalCount int
	// PassRate is the overall DMARC share (dkim=pass OR spf=pass) — the
	// same definition as Statistics.PassRate, here per source instead of
	// global.
	PassRate float64
	// DKIMPassRate and SPFPassRate are reported separately (unlike
	// PassRate, which OR-combines both): a source with a high
	// DKIMPassRate but a low SPFPassRate still passes DMARC overall (SPF
	// isn't required for that), but the pattern is typical for mail
	// forwarding (the DKIM signature survives forwarding, SPF almost
	// always breaks because the forwarding IP isn't in the original
	// domain's SPF record) — the basis for the classification in
	// internal/web/handlers_sources.go.
	DKIMPassRate float64
	SPFPassRate  float64
	FirstSeen    time.Time
	LastSeen     time.Time
	// Enrichment starts out empty — Enricher.Enrich() fills it in, only
	// in the application layer (internal/app/sourcestats), not in this
	// repository: SQL aggregation and network enrichment are two
	// different kinds of I/O that shouldn't be mixed into one adapter.
	Enrichment Enrichment
}

// SortField is a sort key for Query.
type SortField string

// Sort keys for Query.SortField.
const (
	SortByVolume SortField = "volume"
	SortByIP     SortField = "source_ip"
)

// Query narrows down the sending-sources aggregation and paginates via a
// keyset cursor — the same implementation as report.Query (see
// UMSETZUNGSPLAN.md section 3.3).
type Query struct {
	Period *report.DateRange
	Domain string
	// SortField defaults to SortByVolume (largest source first) — the
	// most useful order for this view from a business perspective.
	SortField SortField
	// Limit caps the page size. 0 means: the adapter's default size.
	Limit int
	// Cursor is an opaque keyset cursor from Page.NextCursor, empty for
	// the first page.
	Cursor string
}

// Page is a page of aggregated sending sources.
type Page struct {
	Stats []Stat
	// NextCursor is empty when no further page exists.
	NextCursor string
}

// Repository is the port for aggregating records by source IP.
// Implemented against SQLite (internal/infra/sqlite.SourceStatsRepository)
// via SQL aggregation, analogous to analysis.Repository.
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}

// Enrichment is additional knowledge about a source IP that doesn't come
// from the reports themselves (FEATURES.md proposals 11.2 "rDNS/PTR
// resolution" and 11.3 "known service recognition").
type Enrichment struct {
	// Hostname is the result of the PTR resolution, empty if none
	// succeeded.
	Hostname string
	// Service is the name of a recognized known service (e.g. "Google
	// Workspace"), empty if none was recognized.
	Service string
}

// Enricher enriches a source IP with a hostname and recognized service.
// No error return value: an unresolvable PTR or an unrecognized service
// are normal, expected outcomes (empty Enrichment), not an error
// condition — the caller then simply shows "—" instead of having to
// handle an error on every call. Implementations cache internally, since
// PTR resolution is network-bound and rarely changes (see
// internal/infra/sourceinfo).
type Enricher interface {
	Enrich(ctx context.Context, ip report.SourceIP) Enrichment
}
