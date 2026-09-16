// Package report enthält das Aggregate Root AggregateReport und seine
// Value Objects — reine Fachlogik, keine Abhängigkeiten außerhalb der
// Standardbibliothek (IMPLEMENTIERUNG.md Abschnitt 6.1).
package report

import (
	"strings"
	"time"
)

// ReportID ist der surrogate Primärschlüssel eines gespeicherten Reports,
// vergeben von der Persistenzschicht. Der Nullwert bedeutet "noch nicht
// gespeichert" — Parser und andere Erzeuger außerhalb der Persistenz lassen
// dieses Feld unbesetzt.
//
// Umbenennung zu "ID" würde mit dem Feld AggregateReport.ID kollidieren
// (Feld und Typ hießen dann identisch "ID ID") — schlechter lesbar als
// der bewusst in Kauf genommene Stutter.
//
//nolint:revive // "ReportID" stuttert als report.ReportID, aber eine
type ReportID int64

// Metadata sind die Rahmendaten eines Reports (Element "report_metadata"):
// wer berichtet, worüber, für welchen Zeitraum.
type Metadata struct {
	OrgName          string
	Email            string
	ExtraContactInfo string
	// ReportID ist die vom berichtenden Empfänger vergebene Kennung
	// (Element "report_id") — ein String, nicht zu verwechseln mit dem
	// surrogate AggregateReport.ID.
	ReportID string
	Range    DateRange
	Errors   []string
}

// SourceReference beschreibt die Herkunft eines Reports: über welches
// Konto und welche Nachricht er ins System gelangt ist.
type SourceReference struct {
	AccountID  string
	Mailbox    string
	MessageUID uint32
	Filename   string
}

// AggregateReport ist das Aggregate Root eines DMARC-Berichts. Records
// existieren nur im Kontext ihres Reports und werden ausschließlich über
// ihn geladen und gespeichert (IMPLEMENTIERUNG.md Abschnitt 6.1).
type AggregateReport struct {
	ID         ReportID
	Metadata   Metadata
	Policy     PublishedPolicy
	Records    []Record
	ImportedAt time.Time
	SourceRef  SourceReference
}

// NewAggregateReport erzwingt die Invarianten aus IMPLEMENTIERUNG.md
// Abschnitt 6.2: ein Report ohne ReportID oder ohne gültigen Zeitraum ist
// ungültig. Count- und Percentage-Invarianten sind bereits durch NewRecord
// und NewPublishedPolicy erzwungen, bevor ihre Werte hier ankommen.
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

// Key liefert die fachliche Identität des Reports für Deduplizierung
// (IMPLEMENTIERUNG.md Abschnitt 6.3): (OrgName, ReportID, DateRange.Begin).
func (r *AggregateReport) Key() Key {
	return Key{
		OrgName:   r.Metadata.OrgName,
		ReportID:  r.Metadata.ReportID,
		DateBegin: r.Metadata.Range.Begin,
	}
}
