// Package failedrecords enthält die berichtsübergreifende Sicht auf
// einzelne Records, bei denen DMARC nicht bestanden wurde (weder DKIM
// noch SPF aligned, siehe report.PolicyEvaluation.PassesDMARC) —
// dieselbe Idee wie internal/domain/domainstats/sources (dort
// aggregiert), hier aber unaggregiert: eine Zeile je fehlgeschlagenem
// Record, mit allen Rohdaten (rec.Auth, rec.Identifiers,
// rec.Evaluated.Reasons). Nötig, weil report.Repository.Query bewusst
// berichtsweise arbeitet (eine Zeile je Report, siehe dortige
// Dokumentation) — Records einzelner Berichte liefert bisher nur
// FindByID, nie berichtsübergreifend gefiltert/sortiert/paginiert.
package failedrecords

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Record ist ein einzelner, berichtsübergreifend gefundener Datensatz,
// bei dem DMARC nicht bestanden wurde — mit genug Kontext (Organisation,
// Zeitraum, Report-ID) für einen Link zurück auf den vollständigen
// Bericht, plus den vollen Rohdaten für die Detailanzeige.
type Record struct {
	ReportID     report.ReportID
	OrgName      string
	PolicyDomain report.DomainName
	PeriodBegin  time.Time
	PeriodEnd    time.Time

	SourceIP    report.SourceIP
	Count       int
	Disposition report.Disposition
	DKIM        report.AuthResultValue
	SPF         report.AuthResultValue

	Identifiers report.Identifiers
	Auth        report.AuthResults
	Reasons     []report.PolicyOverrideReason
}

// SortField ist ein Sortierschlüssel für Query.
type SortField string

// Sortierschlüssel für Query.SortField. SortByDate ist die Voreinstellung
// (neuester Bericht zuerst) — SortBySourceIP gruppiert wiederholte
// Fehlschläge desselben Absenders sichtbar nebeneinander.
const (
	SortByDate     SortField = "date_begin"
	SortBySourceIP SortField = "source_ip"
)

// Query grenzt die Fehlschläge-Suche ein und paginiert per Keyset-Cursor
// — dieselbe Umsetzung wie domainstats.Query/sources.Query.
type Query struct {
	Period   *report.DateRange
	Domain   string
	SourceIP string

	// SortField ist standardmäßig SortByDate.
	SortField SortField

	// Limit begrenzt die Seitengröße. 0 bedeutet: Standardgröße des
	// Adapters.
	Limit int
	// Cursor ist ein opaker Keyset-Cursor aus Page.NextCursor, leer für
	// die erste Seite.
	Cursor string
}

// Page ist eine Seite fehlgeschlagener Records.
type Page struct {
	Records []Record
	// NextCursor ist leer, wenn keine weitere Seite existiert.
	NextCursor string
}

// Repository ist der Port zur berichtsübergreifenden Suche nach
// fehlgeschlagenen Records. Implementiert gegen SQLite
// (internal/infra/sqlite.FailedRecordsRepository).
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}
