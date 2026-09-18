// Package sources enthält die nach Quell-IP aggregierte Sicht auf
// Sendequellen (IMPLEMENTIERUNG.md Abschnitt 10.1: "Sendequellen —
// Aggregiert nach Quell-IP: Volumen, Pass-Rate, PTR/rDNS, erkannter
// Dienst") sowie die Ports dafür.
package sources

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Stat ist die über alle Reports im gewählten Zeitraum aggregierte Sicht
// auf eine einzelne Quell-IP.
type Stat struct {
	SourceIP   report.SourceIP
	TotalCount int
	// PassRate ist der DMARC-Gesamtanteil (dkim=pass ODER spf=pass) —
	// dieselbe Definition wie Statistics.PassRate, hier je Quelle statt
	// global.
	PassRate float64
	// DKIMPassRate und SPFPassRate sind getrennt ausgewiesen (anders als
	// PassRate, das beide ODER-verknüpft): eine Quelle mit hoher
	// DKIMPassRate aber niedriger SPFPassRate besteht DMARC zwar
	// trotzdem (SPF ist dafür nicht nötig), das Muster ist aber
	// typisch für Mail-Weiterleitung (DKIM-Signatur übersteht die
	// Weiterleitung, SPF bricht fast immer, weil die weiterleitende IP
	// nicht im SPF-Record der ursprünglichen Domain steht) — Grundlage
	// für die Einordnung in internal/web/handlers_sources.go.
	DKIMPassRate float64
	SPFPassRate  float64
	FirstSeen    time.Time
	LastSeen     time.Time
	// Enrichment ist zunächst leer — Enricher.Enrich() füllt es ein,
	// erst in der Anwendungsschicht (internal/app/sourcestats), nicht in
	// diesem Repository: SQL-Aggregation und Netzwerk-Anreicherung sind
	// zwei unterschiedliche I/O-Arten, die nicht in einem Adapter
	// vermischt werden sollen.
	Enrichment Enrichment
}

// SortField ist ein Sortierschlüssel für Query.
type SortField string

// Sortierschlüssel für Query.SortField.
const (
	SortByVolume SortField = "volume"
	SortByIP     SortField = "source_ip"
)

// Query grenzt die Sendequellen-Aggregation ein und paginiert per
// Keyset-Cursor — dieselbe Umsetzung wie report.Query (siehe
// UMSETZUNGSPLAN.md Abschnitt 3.3).
type Query struct {
	Period *report.DateRange
	Domain string
	// SortField ist standardmäßig SortByVolume (größte Quelle zuerst) —
	// das ist die fachlich interessanteste Reihenfolge für diese Ansicht.
	SortField SortField
	// Limit begrenzt die Seitengröße. 0 bedeutet: Standardgröße des
	// Adapters.
	Limit int
	// Cursor ist ein opaker Keyset-Cursor aus Page.NextCursor, leer für
	// die erste Seite.
	Cursor string
}

// Page ist eine Seite aggregierter Sendequellen.
type Page struct {
	Stats []Stat
	// NextCursor ist leer, wenn keine weitere Seite existiert.
	NextCursor string
}

// Repository ist der Port zur Aggregation von Records nach Quell-IP.
// Implementiert gegen SQLite (internal/infra/sqlite.SourceStatsRepository)
// per SQL-Aggregation, analog zu analysis.Repository.
type Repository interface {
	Query(ctx context.Context, q Query) (Page, error)
}

// Enrichment ist zusätzliches, nicht aus den Reports selbst stammendes
// Wissen über eine Quell-IP (FEATURES.md Vorschläge 11.2 "rDNS-/
// PTR-Auflösung" und 11.3 "Erkennung bekannter Dienste").
type Enrichment struct {
	// Hostname ist das Ergebnis der PTR-Auflösung, leer wenn keine
	// erfolgreich war.
	Hostname string
	// Service ist der Name eines erkannten bekannten Dienstes (z. B.
	// "Google Workspace"), leer wenn keiner erkannt wurde.
	Service string
}

// Enricher reichert eine Quell-IP mit Hostname und erkanntem Dienst an.
// Kein Fehlerrückgabewert: eine nicht auflösbare PTR oder ein nicht
// erkannter Dienst sind normale, erwartete Ausgänge (leeres Enrichment),
// keine Fehlerbedingung — der Aufrufer zeigt dann einfach "—" an, statt
// bei jedem Aufruf Fehlerbehandlung betreiben zu müssen. Implementierungen
// cachen intern, da PTR-Auflösung netzwerkgebunden ist und sich selten
// ändert (siehe internal/infra/sourceinfo).
type Enricher interface {
	Enrich(ctx context.Context, ip report.SourceIP) Enrichment
}
