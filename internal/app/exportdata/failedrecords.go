package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

// WriteFailedRecordsCSV exportiert eine Liste fehlgeschlagener Records
// (z. B. das Ergebnis einer queryfailedrecords.UseCase.List) als CSV —
// eine Zeile je Record, passend zur Fehlschläge-Ansicht.
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
		return fmt.Errorf("csv konnte nicht vollständig geschrieben werden: %w", err)
	}
	return nil
}

// WriteFailedRecordsCSVHeader schreibt die Kopfzeile für
// WriteFailedRecordCSVRow — beide einzeln exportiert für einen Aufrufer,
// der viele Seiten (Keyset-Pagination) nacheinander in denselben
// csv.Writer schreiben will (siehe WriteReportsCSVHeader).
func WriteFailedRecordsCSVHeader(cw *csv.Writer) error {
	header := []string{
		"OrgName", "ReportID", "PolicyDomain", "PeriodBegin", "PeriodEnd",
		"SourceIP", "MessageCount", "Disposition", "DKIM", "SPF",
		"HeaderFrom", "EnvelopeFrom", "EnvelopeTo",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv-kopfzeile konnte nicht geschrieben werden: %w", err)
	}
	return nil
}

// WriteFailedRecordCSVRow schreibt eine einzelne fehlgeschlagene
// Record-Zeile passend zur Kopfzeile aus WriteFailedRecordsCSVHeader —
// wie WriteRecordsCSV (Ein-Bericht-Export) auf die aligned Zusammenfassung
// beschränkt, ohne die rohen Mehrfach-DKIM-/SPF-Ergebnisse: eine flache
// CSV-Zeile kann diese nicht sinnvoll abbilden, die vollen Rohdaten bleiben
// der Web-Detailansicht vorbehalten.
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
		return fmt.Errorf("csv-zeile für quell-ip %q konnte nicht geschrieben werden: %w", rec.SourceIP.String(), err)
	}
	return nil
}
