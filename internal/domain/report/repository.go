package report

import (
	"context"
	"errors"
	"time"
)

// ErrDuplicate is returned by Save when a report with the same business
// identity (Key) is already saved. Defined as a sentinel error in the
// domain port, not in the SQLite adapter — callers from the application
// layer (e.g. syncreports) may depend on ports, but not on concrete infra
// packages (DIP, IMPLEMENTIERUNG.md section 4.2). A caller usually checks
// beforehand via Exists — this error is the last line of defense against
// a race condition between Exists and Save.
var ErrDuplicate = errors.New("a report with this org_name/report_id/date_begin combination already exists")

// Key is the business identity of a report: (OrgName, ReportID,
// DateRange.Begin) — see IMPLEMENTIERUNG.md section 6.3. The basis for
// deduplication during import.
type Key struct {
	OrgName   string
	ReportID  string
	DateBegin time.Time
}

// SortField is a sort key for Query.
type SortField string

// Sort keys for Query.SortField.
const (
	SortByDateBegin SortField = "date_begin"
	SortByOrgName   SortField = "org_name"
	SortByDomain    SortField = "domain"
)

// SortDirection is the sort direction for Query.
type SortDirection string

// Sort directions for Query.SortDirection.
const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

// GroupBy is the grouping dimension for Query.
type GroupBy string

// Grouping dimensions for Query.GroupBy. Acts as an additional, primary
// sort key (same group stands together), not as a SQL aggregation —
// Query still returns a page of AggregateReport, not aggregated metrics.
// Real aggregations (metrics, grouping over records instead of reports)
// are a separate, application-side use case (work package 4/6), not part
// of this port.
const (
	GroupByNone   GroupBy = ""
	GroupByDomain GroupBy = "domain"
	GroupByOrg    GroupBy = "org"
	// GroupBySourceIP groups by a record's source IP — that's a property
	// of records, not of reports, and can't be meaningfully mapped onto a
	// page of AggregateReport values. Adapters reject this value with an
	// error (see reportquery.go).
	GroupBySourceIP GroupBy = "source_ip"
)

// Query filters, sorts and groups saved reports. The implementation
// (filtering/sorting/grouping in SQL instead of in Go, keyset instead of
// offset pagination) is fixed in UMSETZUNGSPLAN.md section 3.3 and
// concerns the adapter (work package 2), not this port.
type Query struct {
	Period        *DateRange
	Domain        string
	OrgName       string
	SourceIP      string
	Disposition   Disposition
	SortField     SortField
	SortDirection SortDirection
	GroupBy       GroupBy
	// Limit caps the page size. 0 means: the adapter's default size.
	Limit int
	// Cursor is an opaque keyset cursor from Page.NextCursor, empty for
	// the first page.
	Cursor string
}

// Page is a page of query results.
type Page struct {
	Reports []AggregateReport
	// NextCursor is empty when no further page exists.
	NextCursor string
}

// Repository is the port for persisting AggregateReport
// (IMPLEMENTIERUNG.md section 6.4). Implemented against SQLite in work
// package 2.
type Repository interface {
	// Save saves a report completely or not at all (transaction).
	Save(ctx context.Context, r *AggregateReport) error
	// Exists checks the business identity, the basis of deduplication.
	Exists(ctx context.Context, key Key) (bool, error)
	// FindByID loads a report completely, including all records — for the
	// report detail view (IMPLEMENTIERUNG.md section 10.1).
	FindByID(ctx context.Context, id ReportID) (*AggregateReport, error)
	// Query returns a page of reports for the reports table. The returned
	// AggregateReport values deliberately have an empty Records field: the
	// table shows one row per report, not per record, loading all records
	// of every visible page would be pure waste (see IMPLEMENTIERUNG.md
	// section 10.4 on the lazy data source). FindByID loads the records of
	// a single report.
	Query(ctx context.Context, q Query) (Page, error)
}

// Pruner is an optional additional port to Repository for the retention
// policy (work package 7, IMPLEMENTIERUNG.md O-7) — deliberately separate
// from Repository instead of another method there: deleting by age is a
// pure maintenance operation that only internal/app/retention needs, not
// every caller of Repository (the same slicing idea as
// domain/sync.MultiReportParser/MailboxLister). sqlite.ReportRepository
// implements Pruner in addition to Repository.
type Pruner interface {
	// DeleteOlderThan deletes all reports whose report period ends
	// entirely before cutoff (DateRange.End < cutoff), and returns the
	// number of deleted reports. Also deletes records, report_errors and
	// raw_reports via the ON DELETE CASCADE foreign keys.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
