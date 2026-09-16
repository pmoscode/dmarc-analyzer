// Package analysis enthält die Kennzahlen für das Dashboard
// (IMPLEMENTIERUNG.md Abschnitt 10.2) sowie den Port, über den sie
// berechnet werden.
package analysis

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Query grenzt die Statistics-Berechnung ein.
type Query struct {
	Period report.DateRange
	// Domain filtert optional auf eine veröffentlichte Policy-Domain.
	// Leer bedeutet: alle Domains.
	Domain string
}

// Statistics sind die in IMPLEMENTIERUNG.md Abschnitt 10.2 aufgeführten
// Dashboard-Kennzahlen für einen Zeitraum. Zählungen sind Nachrichten
// (Summe von Record.Count), nicht Records — ein Record mit Count=50 sind
// 50 Nachrichten von dieser Quelle.
type Statistics struct {
	// TotalMessages ist die Gesamtzahl ausgewerteter Nachrichten im
	// Zeitraum (Summe aller Record.Count).
	TotalMessages int
	// PassRate ist der Anteil der Nachrichten, die PassesDMARC() erfüllen
	// (dkim=pass oder spf=pass nach Alignment), in [0, 1]. 0, wenn
	// TotalMessages 0 ist.
	PassRate float64
	// SPFAlignmentRate und DKIMAlignmentRate sind getrennt ausgewiesen
	// (IMPLEMENTIERUNG.md Abschnitt 10.2: "SPF-Alignment-Rate und
	// DKIM-Alignment-Rate getrennt").
	SPFAlignmentRate  float64
	DKIMAlignmentRate float64
	// DistinctSources ist die Anzahl unterschiedlicher Quell-IPs.
	DistinctSources int
	// VolumeByDisposition ist die Nachrichtenzahl je Disposition.
	VolumeByDisposition map[report.Disposition]int
}

// Repository ist der Port zur Berechnung von Statistics. Implementiert in
// AP 4 gegen SQLite (internal/infra/sqlite.StatisticsRepository) mit
// SQL-Aggregationen statt Go-seitiger Berechnung über geladene Records —
// die in AP 2 bewusst zurückgestellte Aggregation (siehe
// UMSETZUNGSPLAN.md AP 2) landet hier, zugeschnitten auf genau diesen
// Anwendungsfall statt als generische Erweiterung von report.Query.
type Repository interface {
	Compute(ctx context.Context, q Query) (Statistics, error)
	// DailyVolumes liefert das Nachrichtenvolumen je Tag im Zeitraum von
	// q, sortiert aufsteigend nach Tag — für die Zeitreihe (Abschnitt
	// 10.3).
	DailyVolumes(ctx context.Context, q Query) ([]DailyVolume, error)
	// TopSources liefert die nach Volumen absteigend sortierten
	// Sendequellen im Zeitraum von q, begrenzt auf limit Einträge.
	TopSources(ctx context.Context, q Query, limit int) ([]SourceVolume, error)
	// Heatmap liefert die Quelle-×-Tag-Matrix für die sourceLimit
	// volumenstärksten Quellen im Zeitraum von q (dieselbe Reihenfolge
	// wie TopSources mit demselben Limit).
	Heatmap(ctx context.Context, q Query, sourceLimit int) (Heatmap, error)
}
