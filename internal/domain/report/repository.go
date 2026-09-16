package report

import (
	"context"
	"time"
)

// Key ist die fachliche Identität eines Reports:
// (OrgName, ReportID, DateRange.Begin) — siehe IMPLEMENTIERUNG.md
// Abschnitt 6.3. Grundlage für Deduplizierung beim Import.
type Key struct {
	OrgName   string
	ReportID  string
	DateBegin time.Time
}

// SortField ist ein Sortierschlüssel für Query.
type SortField string

// Sortierschlüssel für Query.SortField.
const (
	SortByDateBegin SortField = "date_begin"
	SortByOrgName   SortField = "org_name"
	SortByDomain    SortField = "domain"
)

// SortDirection ist die Sortierrichtung für Query.
type SortDirection string

// Sortierrichtungen für Query.SortDirection.
const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

// GroupBy ist die Gruppierungsdimension für Query.
type GroupBy string

// Gruppierungsdimensionen für Query.GroupBy.
const (
	GroupByNone     GroupBy = ""
	GroupByDomain   GroupBy = "domain"
	GroupByOrg      GroupBy = "org"
	GroupBySourceIP GroupBy = "source_ip"
)

// Query filtert, sortiert und gruppiert gespeicherte Reports. Die Umsetzung
// (Filtern/Sortieren/Gruppieren in SQL statt in Go, Keyset- statt
// Offset-Pagination) ist in UMSETZUNGSPLAN.md Abschnitt 3.3 festgelegt und
// betrifft den Adapter (AP 2), nicht diesen Port.
type Query struct {
	Period        *DateRange
	Domain        string
	OrgName       string
	SourceIP      string
	Disposition   Disposition
	SortField     SortField
	SortDirection SortDirection
	GroupBy       GroupBy
	// Limit begrenzt die Seitengröße. 0 bedeutet: Standardgröße des Adapters.
	Limit int
	// Cursor ist ein opaker Keyset-Cursor aus Page.NextCursor, leer für die
	// erste Seite.
	Cursor string
}

// Page ist eine Seite von Abfrageergebnissen.
type Page struct {
	Reports []AggregateReport
	// NextCursor ist leer, wenn keine weitere Seite existiert.
	NextCursor string
}

// Repository ist der Port zur Persistenz von AggregateReport
// (IMPLEMENTIERUNG.md Abschnitt 6.4). Implementiert in AP 2 gegen SQLite.
type Repository interface {
	// Save speichert einen Report vollständig oder gar nicht (Transaktion).
	Save(ctx context.Context, r *AggregateReport) error
	// Exists prüft die fachliche Identität, Grundlage der Deduplizierung.
	Exists(ctx context.Context, key Key) (bool, error)
	FindByID(ctx context.Context, id ReportID) (*AggregateReport, error)
	Query(ctx context.Context, q Query) (Page, error)
}
