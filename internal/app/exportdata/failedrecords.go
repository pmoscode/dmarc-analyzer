package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

// WriteFailedRecordsCSV exports a list of failed records (e.g. the
// result of a queryfailedrecords.UseCase.List) as CSV — one row per
// record, matching the failures view.
func WriteFailedRecordsCSV(w io.Writer, records []failedrecords.Record) error {
	cw := csv.NewWriter(w)
	if err := WriteFailedRecordsCSVHeader(cw); err != nil {
		return err
	}
	for _, rec := range records {
		if err := WriteFailedRecordCSVRow(cw, rec); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv could not be fully written: %w", err)
	}
	return nil
}

// WriteFailedRecordsCSVHeader writes the header row for
// WriteFailedRecordCSVRow — both exported separately for a caller that
// wants to write many pages (keyset pagination) one after another into
// the same csv.Writer (see WriteReportsCSVHeader).
func WriteFailedRecordsCSVHeader(cw *csv.Writer) error {
	header := []string{
		"OrgName", "ReportID", "PolicyDomain", "PeriodBegin", "PeriodEnd",
		"SourceIP", "MessageCount", "Disposition", "DKIM", "SPF",
		"HeaderFrom", "EnvelopeFrom", "EnvelopeTo",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header could not be written: %w", err)
	}
	return nil
}

// WriteFailedRecordCSVRow writes a single failed-record row matching the
// header from WriteFailedRecordsCSVHeader — like WriteRecordsCSV
// (single-report export), limited to the aligned summary, without the
// raw multiple DKIM/SPF results: a flat CSV row can't meaningfully
// represent those, the full raw data remains reserved for the web detail
// view.
func WriteFailedRecordCSVRow(cw *csv.Writer, rec failedrecords.Record) error {
	row := []string{
		rec.OrgName,
		strconv.FormatInt(int64(rec.ReportID), 10),
		rec.PolicyDomain.String(),
		formatCSVTime(rec.PeriodBegin),
		formatCSVTime(rec.PeriodEnd),
		rec.SourceIP.String(),
		strconv.Itoa(rec.Count),
		string(rec.Disposition),
		string(rec.DKIM),
		string(rec.SPF),
		rec.Identifiers.HeaderFrom.String(),
		rec.Identifiers.EnvelopeFrom,
		rec.Identifiers.EnvelopeTo,
	}
	if err := cw.Write(row); err != nil {
		return fmt.Errorf("csv row for source IP %q could not be written: %w", rec.SourceIP.String(), err)
	}
	return nil
}
