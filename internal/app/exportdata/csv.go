// Package exportdata exportiert gefilterte Ansichten als CSV
// (FEATURES.md Vorschlag 11.4). Der PNG-Export von Diagrammen ist
// bewusst nicht Teil von AP 4: er braucht den ChartRenderer-Port, der
// erst in AP 6 entsteht (siehe UMSETZUNGSPLAN.md).
//
// Reine Formatierungsfunktionen ohne eigene Ports: sie schreiben in einen
// vom Aufrufer übergebenen io.Writer (Datei, HTTP-Response, Puffer für
// einen Zwischenspeicher-Export) und lösen selbst keine I/O aus.
package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// WriteReportsCSV exportiert eine Liste von Reports (z. B. das Ergebnis
// einer queryreports.List) als CSV — eine Zeile pro Report, passend zur
// Berichtstabelle. Erwartet keine geladenen Records (siehe
// report.Repository.Query-Dokumentation).
func WriteReportsCSV(w io.Writer, reports []report.AggregateReport) error {
	cw := csv.NewWriter(w)

	header := []string{
		"OrgName", "ReportID", "Domain", "PeriodBegin", "PeriodEnd",
		"Policy", "SubdomainPolicy", "Percentage", "DKIMAlignment", "SPFAlignment",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv-kopfzeile konnte nicht geschrieben werden: %w", err)
	}

	for _, r := range reports {
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
			return fmt.Errorf("csv-zeile für report %q konnte nicht geschrieben werden: %w", r.Metadata.ReportID, err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv konnte nicht vollständig geschrieben werden: %w", err)
	}
	return nil
}

// WriteRecordsCSV exportiert die Records eines einzelnen Reports (die
// Bericht-Detailansicht) als CSV — eine Zeile pro Sendequelle.
func WriteRecordsCSV(w io.Writer, r *report.AggregateReport) error {
	cw := csv.NewWriter(w)

	header := []string{
		"SourceIP", "MessageCount", "Disposition", "DKIM", "SPF", "HeaderFrom", "EnvelopeFrom", "EnvelopeTo",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv-kopfzeile konnte nicht geschrieben werden: %w", err)
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
			return fmt.Errorf("csv-zeile für quell-ip %q konnte nicht geschrieben werden: %w", rec.SourceIP.String(), err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csv konnte nicht vollständig geschrieben werden: %w", err)
	}
	return nil
}

func formatCSVTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
