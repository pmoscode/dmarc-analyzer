package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// WriteSourceStatsCSV exportiert eine Seite aggregierter Sendequellen
// (z. B. das Ergebnis einer sourcestats.UseCase.List) als CSV — eine
// Zeile pro Quell-IP, passend zur Sendequellen-Ansicht.
func WriteSourceStatsCSV(w io.Writer, stats []sources.Stat) error {
	cw := csv.NewWriter(w)

	header := []string{"SourceIP", "TotalCount", "PassRate", "Hostname", "Service", "FirstSeen", "LastSeen"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv-kopfzeile konnte nicht geschrieben werden: %w", err)
	}

	for _, s := range stats {
		row := []string{
			s.SourceIP.String(),
			strconv.Itoa(s.TotalCount),
			strconv.FormatFloat(s.PassRate, 'f', 4, 64),
			s.Enrichment.Hostname,
			s.Enrichment.Service,
			formatCSVTime(s.FirstSeen),
			formatCSVTime(s.LastSeen),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csv-zeile für quell-ip %q konnte nicht geschrieben werden: %w", s.SourceIP.String(), err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv konnte nicht vollständig geschrieben werden: %w", err)
	}
	return nil
}
