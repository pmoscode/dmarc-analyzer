// Package domainstats contains the view of incoming reports aggregated by
// published policy domain — the same idea as internal/domain/sources
// (aggregation by source IP there), one row per domain here instead of
// one row per report (see /berichte).
package domainstats

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Stat is the view of a single published policy domain, aggregated across
// all reports in the selected period.
type Stat struct {
	Domain     report.DomainName
	TotalCount int
	PassRate   float64
	// ReportCount is the number of individual incoming reports that fed
	// into this row — the domain view doesn't replace the reports list
	// (there every report stays its own row, linked to the detail view),
	// it only summarizes; ReportCount shows how much was "absorbed" in
	// the process.
	ReportCount int
	// DistinctSources is the number of distinct source IPs that sent
	// messages for this domain — the same metric as
	// Statistics.DistinctSources, here per domain instead of
	// global/filtered.
	DistinctSources int
	FirstSeen       time.Time
	LastSeen        time.Time
}

// SortField is a sort key for Query.
type SortField string

// Sort keys for Query.SortField.
const (
	SortByVolume SortField = "volume"
	SortByDomain SortField = "domain"
)

// Query narrows down the domain aggregation and paginates via a keyset
// cursor — the same implementation as sources.Query.
type Query struct {
	Period *report.DateRange
	// Domain optionally filters to exactly one policy domain — usually
	// not needed in this already domain-aggregated view (then at most one
	// row remains), but part of the shared filter bar
	// (overview/reports/sending sources/domains).
	Domain string
	// SortField defaults to SortByVolume (largest domain first).
	SortField SortField
	// Limit caps the page size. 0 means: the adapter's default size.
	Limit int
	// Cursor is an opaque keyset cursor from Page.NextCursor, empty for
	// the first page.
	Cursor string
}

// Page is a page of aggregated domains.
type Page struct {
	Stats []Stat
	// NextCursor is empty when no further page exists.
	NextCursor string
}

// Repository is the port for aggregating reports by policy domain.
// Implemented against SQLite (internal/infra/sqlite.DomainStatsRepository)
// via SQL aggregation, analogous to sources.Repository.
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}
