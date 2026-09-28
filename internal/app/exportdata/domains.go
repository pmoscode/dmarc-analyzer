package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

// WriteDomainStatsCSV exports a page of aggregated domains (e.g. the
// result of a domainoverview.UseCase.List) as CSV — one row per domain,
// matching the domains view.
func WriteDomainStatsCSV(w io.Writer, stats []domainstats.Stat) error {
	cw := csv.NewWriter(w)
	if err := WriteDomainStatsCSVHeader(cw); err != nil {
		return err
	}
	for _, s := range stats {
		if err := WriteDomainStatCSVRow(cw, s); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv could not be fully written: %w", err)
	}
	return nil
}

// WriteDomainStatsCSVHeader writes the header row for
// WriteDomainStatCSVRow — see WriteSourceStatsCSVHeader for the same
// rationale (streaming across multiple pages).
func WriteDomainStatsCSVHeader(cw *csv.Writer) error {
	header := []string{"Domain", "TotalCount", "PassRate", "ReportCount", "DistinctSources", "FirstSeen", "LastSeen"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv header could not be written: %w", err)
	}
	return nil
}

// WriteDomainStatCSVRow writes a single domain row matching the header
// from WriteDomainStatsCSVHeader.
func WriteDomainStatCSVRow(cw *csv.Writer, s domainstats.Stat) error {
	row := []string{
		s.Domain.String(),
		strconv.Itoa(s.TotalCount),
		strconv.FormatFloat(s.PassRate, 'f', 4, 64),
		strconv.Itoa(s.ReportCount),
		strconv.Itoa(s.DistinctSources),
		formatCSVTime(s.FirstSeen),
		formatCSVTime(s.LastSeen),
	}
	if err := cw.Write(row); err != nil {
		return fmt.Errorf("csv row for domain %q could not be written: %w", s.Domain.String(), err)
	}
	return nil
}
