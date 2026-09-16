package analysis

import (
	"image"
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
	// zeigt der Chart-Renderer die IP-Adresse selbst). Wird von
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
}

// Heatmap ist die Matrix Quelle × Tag für die Top-Quellen im Zeitraum
// (IMPLEMENTIERUNG.md Abschnitt 10.3: "Heatmap — Sendequelle × Tag, Farbe
// = Pass-Rate"). Cells[i][j] gehört zu Sources[i] am Tag Days[j].
type Heatmap struct {
	Sources []report.SourceIP
	// SourceLabels sind die zu Sources parallelen, für Menschen
	// sprechenden Bezeichnungen (siehe SourceVolume.Label) — leer (oder
	// kürzer als Sources), wenn keine Anreicherung stattgefunden hat;
	// der Chart-Renderer fällt dann auf die IP-Adresse zurück.
	SourceLabels []string
	Days         []time.Time
	Cells        [][]HeatmapCell
}

// ChartRenderer rendert Diagramm-Daten als image.Image zur Einbettung in
// Fyne-Widgets (IMPLEMENTIERUNG.md Abschnitt 8.2: "ChartRenderer als
// Port"). v1 implementiert das gegen go-chart (internal/infra/charts) —
// bei Bedarf für native, interaktive Fyne-Widgets später nur der Adapter
// zu tauschen, ohne dass die Aufrufer (internal/ui/dashboard) sich ändern.
type ChartRenderer interface {
	// DailyVolumeChart zeichnet die gestapelte Zeitreihe.
	DailyVolumeChart(data []DailyVolume) (image.Image, error)
	// TopSourcesChart zeichnet die Top-Sendequellen als Balkendiagramm.
	TopSourcesChart(data []SourceVolume) (image.Image, error)
	// DispositionChart zeichnet die Verteilung der Dispositions als Donut.
	DispositionChart(data map[report.Disposition]int) (image.Image, error)
	// HeatmapChart zeichnet die Quelle-×-Tag-Heatmap.
	HeatmapChart(data Heatmap) (image.Image, error)
}
