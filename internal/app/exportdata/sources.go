package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// WriteSourceStatsCSV exports a page of aggregated sending sources (e.g.
// the result of a sourcestats.UseCase.List) as CSV — one row per source
// IP, matching the sending sources view.
func WriteSourceStatsCSV(w io.Writer, stats []sources.Stat) error {
	cw := csv.NewWriter(w)
	if err := WriteSourceStatsCSVHeader(cw); err != nil {
		return err
	}
	for _, s := range stats {
		if err := WriteSourceStatCSVRow(cw, s); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv could not be fully written: %w", err)
	}
	return nil
}

// WriteSourceStatsCSVHeader writes the header row for
// WriteSourceStatCSVRow — see WriteReportsCSVHeader/WriteReportCSVRow in
// csv.go for the same rationale (streaming across multiple pages).
func WriteSourceStatsCSVHeader(cw *csv.Writer) error {
	header := []string{"SourceIP", "TotalCount", "PassRate", "DKIMPassRate", "SPFPassRate", "Hostname", "Service", "FirstSeen", "LastSeen"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header could not be written: %w", err)
	}
	return nil
}

// WriteSourceStatCSVRow writes a single sending-source row matching the
// header from WriteSourceStatsCSVHeader.
func WriteSourceStatCSVRow(cw *csv.Writer, s sources.Stat) error {
	row := []string{
		s.SourceIP.String(),
		strconv.Itoa(s.TotalCount),
		strconv.FormatFloat(s.PassRate, 'f', 4, 64),
		strconv.FormatFloat(s.DKIMPassRate, 'f', 4, 64),
		strconv.FormatFloat(s.SPFPassRate, 'f', 4, 64),
		s.Enrichment.Hostname,
		s.Enrichment.Service,
		formatCSVTime(s.FirstSeen),
		formatCSVTime(s.LastSeen),
	}
	if err := cw.Write(row); err != nil {
		return fmt.Errorf("csv row for source IP %q could not be written: %w", s.SourceIP.String(), err)
	}
	return nil
}
