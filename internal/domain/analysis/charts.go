package analysis

import (
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// DailyVolume ist das Nachrichtenvolumen eines Tages, aufgeteilt nach
// DMARC-Ergebnis (IMPLEMENTIERUNG.md Abschnitt 10.3: "Nachrichtenvolumen
// pro Tag, gestapelt nach Pass/Fail"). Pass/Fail folgt derselben Definition
// wie Statistics.PassRate (PolicyEvaluation.PassesDMARC()).
type DailyVolume struct {
	// Day ist auf Tagesbeginn (00:00 UTC) normalisiert.
	Day  time.Time
	Pass int
	Fail int
}

// SourceVolume ist das Nachrichtenvolumen einer einzelnen Quell-IP im
// Zeitraum, mit ihrer Pass-Rate (IMPLEMENTIERUNG.md Abschnitt 10.3:
// "Top-10-Sendequellen nach Volumen, eingefärbt nach Pass-Rate").
type SourceVolume struct {
	SourceIP report.SourceIP
	Total    int
	PassRate float64
	// Label ist die für Menschen sprechende Bezeichnung dieser Quelle —
	// erkannter Diensteanbieter, sonst PTR-Hostname, sonst leer (dann
	// zeigt das Diagramm im Browser die IP-Adresse selbst). Wird von
	// app/statistics.UseCase.Dashboard() über sources.Enricher befüllt —
	// dieselbe Anreicherung wie in der Sendequellen-Ansicht
	// (app/sourcestats), hier nur zusätzlich fürs Dashboard-Diagramm.
	Label string
}

// HeatmapCell ist die Pass-Rate einer Quelle an einem Tag. HasData ist
// false, wenn die Quelle an diesem Tag keine Nachrichten gesendet hat —
// eine PassRate von 0 wäre dann irreführend (sähe aus wie "komplett
// fehlgeschlagen" statt "keine Daten").
type HeatmapCell struct {
	PassRate float64
	HasData  bool
	// Total ist die Nachrichtenzahl dieser Quelle an diesem Tag — für die
	// Heatmap-Tooltips im Web-Frontend (MIGRATIONSPLAN.md Abschnitt 6a:
	// "Tooltip mit Quelle, Tag, Pass-Rate und Nachrichtenzahl"). 0, wenn
	// HasData false ist.
	Total int
}

// Heatmap ist die Matrix Quelle × Tag für die Top-Quellen im Zeitraum
// (IMPLEMENTIERUNG.md Abschnitt 10.3: "Heatmap — Sendequelle × Tag, Farbe
// = Pass-Rate"). Cells[i][j] gehört zu Sources[i] am Tag Days[j].
type Heatmap struct {
	Sources []report.SourceIP
	// SourceLabels sind die zu Sources parallelen, für Menschen
	// sprechenden Bezeichnungen (siehe SourceVolume.Label) — leer (oder
	// kürzer als Sources), wenn keine Anreicherung stattgefunden hat;
	// das Diagramm im Browser fällt dann auf die IP-Adresse zurück.
	SourceLabels []string
	Days         []time.Time
	Cells        [][]HeatmapCell
}
