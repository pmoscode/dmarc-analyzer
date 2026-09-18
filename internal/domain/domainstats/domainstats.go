// Package domainstats enthält die nach veröffentlichter Policy-Domain
// aggregierte Sicht auf eingegangene Reports — dieselbe Idee wie
// internal/domain/sources (dort Aggregation nach Quell-IP), hier eine
// Zeile je Domain statt eine Zeile je Report (siehe /berichte).
package domainstats

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Stat ist die über alle Reports im gewählten Zeitraum aggregierte Sicht
// auf eine einzelne veröffentlichte Policy-Domain.
type Stat struct {
	Domain     report.DomainName
	TotalCount int
	PassRate   float64
	// ReportCount ist die Anzahl einzelner eingegangener Reports, die in
	// diese Zeile eingeflossen sind — die Domain-Ansicht ersetzt die
	// Berichte-Liste nicht (dort bleibt jeder Report eine eigene Zeile,
	// verlinkt zur Detailansicht), sie fasst nur zusammen; ReportCount
	// zeigt, wie viel dabei "verschluckt" wurde.
	ReportCount int
	// DistinctSources ist die Anzahl unterschiedlicher Quell-IPs, die für
	// diese Domain Nachrichten verschickt haben — dieselbe Kennzahl wie
	// Statistics.DistinctSources, hier je Domain statt global/gefiltert.
	DistinctSources int
	FirstSeen       time.Time
	LastSeen        time.Time
}

// SortField ist ein Sortierschlüssel für Query.
type SortField string

// Sortierschlüssel für Query.SortField.
const (
	SortByVolume SortField = "volume"
	SortByDomain SortField = "domain"
)

// Query grenzt die Domain-Aggregation ein und paginiert per Keyset-Cursor
// — dieselbe Umsetzung wie sources.Query.
type Query struct {
	Period *report.DateRange
	// Domain filtert optional exakt auf eine Policy-Domain — in dieser
	// bereits nach Domain aggregierten Ansicht meist nicht gebraucht
	// (dann bleibt höchstens eine Zeile übrig), aber Teil der geteilten
	// Filterleiste (Übersicht/Berichte/Sendequellen/Domains).
	Domain string
	// SortField ist standardmäßig SortByVolume (größte Domain zuerst).
	SortField SortField
	// Limit begrenzt die Seitengröße. 0 bedeutet: Standardgröße des
	// Adapters.
	Limit int
	// Cursor ist ein opaker Keyset-Cursor aus Page.NextCursor, leer für
	// die erste Seite.
	Cursor string
}

// Page ist eine Seite aggregierter Domains.
type Page struct {
	Stats []Stat
	// NextCursor ist leer, wenn keine weitere Seite existiert.
	NextCursor string
}

// Repository ist der Port zur Aggregation von Reports nach Policy-Domain.
// Implementiert gegen SQLite (internal/infra/sqlite.DomainStatsRepository)
// per SQL-Aggregation, analog zu sources.Repository.
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}
