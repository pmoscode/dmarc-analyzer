package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

// WriteDomainStatsCSV exportiert eine Seite aggregierter Domains (z. B.
// das Ergebnis einer domainoverview.UseCase.List) als CSV — eine Zeile
// pro Domain, passend zur Domains-Ansicht.
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
		return fmt.Errorf("csv konnte nicht vollständig geschrieben werden: %w", err)
	}
	return nil
}

// WriteDomainStatsCSVHeader schreibt die Kopfzeile für
// WriteDomainStatCSVRow — siehe WriteSourceStatsCSVHeader für dieselbe
// Begründung (Streaming über mehrere Seiten hinweg).
func WriteDomainStatsCSVHeader(cw *csv.Writer) error {
	header := []string{"Domain", "TotalCount", "PassRate", "ReportCount", "DistinctSources", "FirstSeen", "LastSeen"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv-kopfzeile konnte nicht geschrieben werden: %w", err)
	}
	return nil
}

// WriteDomainStatCSVRow schreibt eine einzelne Domain-Zeile passend zur
// Kopfzeile aus WriteDomainStatsCSVHeader.
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
		return fmt.Errorf("csv-zeile für domain %q konnte nicht geschrieben werden: %w", s.Domain.String(), err)
	}
	return nil
}
