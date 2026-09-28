// Package report contains the aggregate root AggregateReport and its
// value objects — pure domain logic, no dependencies outside the standard
// library (IMPLEMENTIERUNG.md section 6.1).
package report

import (
	"strings"
	"time"
)

// ReportID is the surrogate primary key of a stored report, assigned by
// the persistence layer. The zero value means "not yet saved" — parsers
// and other producers outside persistence leave this field unset.
//
// Renaming to "ID" would collide with the AggregateReport.ID field (field
// and type would both be called "ID ID") — less readable than the
// deliberately accepted stutter.
//
//nolint:revive // "ReportID" stutters as report.ReportID, but a
type ReportID int64

// Metadata is the framing data of a report (element "report_metadata"):
// who is reporting, about what, for which period.
type Metadata struct {
	OrgName          string
	Email            string
	ExtraContactInfo string
	// ReportID is the identifier assigned by the reporting recipient
	// (element "report_id") — a string, not to be confused with the
	// surrogate AggregateReport.ID.
	ReportID string
	Range    DateRange
	Errors   []string
}

// SourceReference describes the origin of a report: via which account and
// which message it entered the system.
type SourceReference struct {
	AccountID  string
	Mailbox    string
	MessageUID uint32
	Filename   string
}

// AggregateReport is the aggregate root of a DMARC report. Records only
// exist in the context of their report and are loaded and saved
// exclusively through it (IMPLEMENTIERUNG.md section 6.1).
type AggregateReport struct {
	ID         ReportID
	Metadata   Metadata
	Policy     PublishedPolicy
	Records    []Record
	ImportedAt time.Time
	SourceRef  SourceReference
}

// NewAggregateReport enforces the invariants from IMPLEMENTIERUNG.md
// section 6.2: a report without a ReportID or without a valid date range
// is invalid. Count and percentage invariants are already enforced by
// NewRecord and NewPublishedPolicy before their values arrive here.
func NewAggregateReport(
	metadata Metadata,
	policy PublishedPolicy,
	records []Record,
	sourceRef SourceReference,
	importedAt time.Time,
) (*AggregateReport, error) {
	if strings.TrimSpace(metadata.ReportID) == "" {
		return nil, errReportIDRequired
	}
	if metadata.Range.IsZero() {
		return nil, errDateRangeRequired
	}

	return &AggregateReport{
		Metadata:   metadata,
		Policy:     policy,
		Records:    records,
		ImportedAt: importedAt.UTC(),
		SourceRef:  sourceRef,
	}, nil
}

// Key returns the business identity of the report for deduplication
// (IMPLEMENTIERUNG.md section 6.3): (OrgName, ReportID, DateRange.Begin).
func (r *AggregateReport) Key() Key {
	return Key{
		OrgName:   r.Metadata.OrgName,
		ReportID:  r.Metadata.ReportID,
		DateBegin: r.Metadata.Range.Begin,
	}
}
