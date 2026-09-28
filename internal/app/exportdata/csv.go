// Package exportdata exports filtered views as CSV and charts as PNG
// (FEATURES.md proposal 11.4).
//
// Pure formatting functions without their own ports: they write into an
// io.Writer passed in by the caller (file, HTTP response, buffer for an
// in-memory export) and don't trigger any I/O themselves. The PNG export
// deliberately takes an already-rendered image.Image, not the
// ChartRenderer port itself — rendering is the caller's job
// (internal/ui/dashboard has already produced the image for display
// anyway), this package only handles encoding.
package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// WriteReportsCSV exports a list of reports (e.g. the result of a
// queryreports.List) as CSV — one row per report, matching the reports
// table. Expects no loaded records (see report.Repository.Query
// documentation).
func WriteReportsCSV(w io.Writer, reports []report.AggregateReport) error {
	cw := csv.NewWriter(w)
	if err := WriteReportsCSVHeader(cw); err != nil {
		return err
	}
	for _, r := range reports {
		if err := WriteReportCSVRow(cw, r); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv could not be fully written: %w", err)
	}
	return nil
}

// WriteReportsCSVHeader writes the header row for WriteReportCSVRow —
// both exported separately for a caller that wants to write many pages
// (e.g. via keyset pagination) one after another into the same csv.Writer
// and flush in between, without first collecting the entire filtered
// result set in memory (MIGRATIONSPLAN.md milestone M4: "CSV export ...
// streamed", see internal/web/handlers_export.go).
func WriteReportsCSVHeader(cw *csv.Writer) error {
	header := []string{
		"OrgName", "ReportID", "Domain", "PeriodBegin", "PeriodEnd",
		"Policy", "SubdomainPolicy", "Percentage", "DKIMAlignment", "SPFAlignment",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header could not be written: %w", err)
	}
	return nil
}

// WriteReportCSVRow writes a single report row matching the header from
// WriteReportsCSVHeader.
func WriteReportCSVRow(cw *csv.Writer, r report.AggregateReport) error {
	row := []string{
		r.Metadata.OrgName,
		r.Metadata.ReportID,
		r.Policy.Domain.String(),
		formatCSVTime(r.Metadata.Range.Begin),
		formatCSVTime(r.Metadata.Range.End),
		string(r.Policy.Policy),
		string(r.Policy.SubdomainPolicy),
		strconv.Itoa(r.Policy.Percentage),
		string(r.Policy.DKIMAlignment),
		string(r.Policy.SPFAlignment),
	}
	if err := cw.Write(row); err != nil {
		return fmt.Errorf("csv row for report %q could not be written: %w", r.Metadata.ReportID, err)
	}
	return nil
}

// WriteRecordsCSV exports the records of a single report (the report
// detail view) as CSV — one row per sending source.
func WriteRecordsCSV(w io.Writer, r *report.AggregateReport) error {
	cw := csv.NewWriter(w)

	header := []string{
		"SourceIP", "MessageCount", "Disposition", "DKIM", "SPF", "HeaderFrom", "EnvelopeFrom", "EnvelopeTo",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header could not be written: %w", err)
	}

	for _, rec := range r.Records {
		row := []string{
			rec.SourceIP.String(),
			strconv.Itoa(rec.Count),
			string(rec.Evaluated.Disposition),
			string(rec.Evaluated.DKIM),
			string(rec.Evaluated.SPF),
			rec.Identifiers.HeaderFrom.String(),
			rec.Identifiers.EnvelopeFrom,
			rec.Identifiers.EnvelopeTo,
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csv row for source IP %q could not be written: %w", rec.SourceIP.String(), err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv could not be fully written: %w", err)
	}
	return nil
}

func formatCSVTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
