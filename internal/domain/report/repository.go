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

// Gruppierungsdimensionen für Query.GroupBy. Wirkt als zusätzlicher,
// primärer Sortierschlüssel (gleiche Gruppe steht zusammen), nicht als
// SQL-Aggregation — Query liefert weiterhin eine Seite von AggregateReport,
// keine aggregierten Kennzahlen. Echte Aggregationen (Kennzahlen,
// Gruppierung über Records statt Reports) sind ein eigener,
// anwendungsseitiger Anwendungsfall (AP 4/6), kein Teil dieses Ports.
const (
	GroupByNone   GroupBy = ""
	GroupByDomain GroupBy = "domain"
	GroupByOrg    GroupBy = "org"
	// GroupBySourceIP gruppiert nach Quell-IP eines Records — das ist eine
	// Eigenschaft von Records, nicht von Reports, und lässt sich auf eine
	// Seite von AggregateReport-Werten nicht sinnvoll abbilden. Adapter
	// lehnen diesen Wert mit einem Fehler ab (siehe reportquery.go).
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
	// FindByID lädt einen Report vollständig, inklusive aller Records —
	// für die Bericht-Detailansicht (IMPLEMENTIERUNG.md Abschnitt 10.1).
	FindByID(ctx context.Context, id ReportID) (*AggregateReport, error)
	// Query liefert eine Seite von Reports für die Berichtstabelle. Die
	// zurückgegebenen AggregateReport-Werte haben bewusst ein leeres
	// Records-Feld: die Tabelle zeigt eine Zeile pro Report, nicht pro
	// Record, ein Laden aller Records jeder sichtbaren Seite wäre reine
	// Verschwendung (siehe IMPLEMENTIERUNG.md Abschnitt 10.4 zur
	// Lazy-Datenquelle). Records eines einzelnen Reports lädt FindByID.
	Query(ctx context.Context, q Query) (Page, error)
}
