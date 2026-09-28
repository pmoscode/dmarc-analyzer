// Package failedrecords contains the cross-report view of individual
// records where DMARC failed (neither DKIM nor SPF aligned, see
// report.PolicyEvaluation.PassesDMARC) — the same idea as
// internal/domain/domainstats/sources (aggregated there), but
// unaggregated here: one row per failed record, with all the raw data
// (rec.Auth, rec.Identifiers, rec.Evaluated.Reasons). Needed because
// report.Repository.Query deliberately works per-report (one row per
// report, see its documentation) — records of individual reports have so
// far only been available via FindByID, never filtered/sorted/paginated
// across reports.
package failedrecords

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Record is a single record found across reports where DMARC failed —
// with enough context (organization, period, report ID) for a link back
// to the full report, plus the full raw data for the detail view.
type Record struct {
	ReportID     report.ReportID
	OrgName      string
	PolicyDomain report.DomainName
	PeriodBegin  time.Time
	PeriodEnd    time.Time

	SourceIP    report.SourceIP
	Count       int
	Disposition report.Disposition
	DKIM        report.AuthResultValue
	SPF         report.AuthResultValue

	Identifiers report.Identifiers
	Auth        report.AuthResults
	Reasons     []report.PolicyOverrideReason
}

// SortField is a sort key for Query.
type SortField string

// Sort keys for Query.SortField. SortByDate is the default (newest report
// first) — SortBySourceIP visibly groups repeated failures from the same
// sender next to each other.
const (
	SortByDate     SortField = "date_begin"
	SortBySourceIP SortField = "source_ip"
)

// Query narrows down the failure search and paginates via a keyset
// cursor — the same implementation as domainstats.Query/sources.Query.
type Query struct {
	Period   *report.DateRange
	Domain   string
	SourceIP string

	// SortField defaults to SortByDate.
	SortField SortField

	// Limit caps the page size. 0 means: the adapter's default size.
	Limit int
	// Cursor is an opaque keyset cursor from Page.NextCursor, empty for
	// the first page.
	Cursor string
}

// Page is a page of failed records.
type Page struct {
	Records []Record
	// NextCursor is empty when no further page exists.
	NextCursor string
}

// Repository is the port for the cross-report search for failed records.
// Implemented against SQLite
// (internal/infra/sqlite.FailedRecordsRepository).
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}
