package analysis

import (
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// DailyVolume is the message volume of one day, split by DMARC result
// (IMPLEMENTIERUNG.md section 10.3: "message volume per day, stacked by
// pass/fail"). Pass/fail follows the same definition as
// Statistics.PassRate (PolicyEvaluation.PassesDMARC()).
type DailyVolume struct {
	// Day is normalized to the start of the day (00:00 UTC).
	Day  time.Time
	Pass int
	Fail int
}

// SourceVolume is the message volume of a single source IP in the period,
// with its pass rate (IMPLEMENTIERUNG.md section 10.3: "top 10 sending
// sources by volume, colored by pass rate").
type SourceVolume struct {
	SourceIP report.SourceIP
	Total    int
	PassRate float64
	// Label is the human-readable name of this source — a recognized
	// service provider, otherwise the PTR hostname, otherwise empty (in
	// which case the chart in the browser shows the IP address itself).
	// Populated by app/statistics.UseCase.Dashboard() via
	// sources.Enricher — the same enrichment as in the sending-sources
	// view (app/sourcestats), just additionally for the dashboard chart
	// here.
	Label string
}

// HeatmapCell is the pass rate of a source on one day. HasData is false
// when the source sent no messages that day — a PassRate of 0 would then
// be misleading (would look like "completely failed" instead of "no
// data").
type HeatmapCell struct {
	PassRate float64
	HasData  bool
	// Total is the message count of this source on this day — for the
	// heatmap tooltips in the web frontend (MIGRATIONSPLAN.md section 6a:
	// "tooltip with source, day, pass rate and message count"). 0 when
	// HasData is false.
	Total int
}

// Heatmap is the source × day matrix for the top sources in the period
// (IMPLEMENTIERUNG.md section 10.3: "heatmap — sending source × day,
// color = pass rate"). Cells[i][j] belongs to Sources[i] on day Days[j].
type Heatmap struct {
	Sources []report.SourceIP
	// SourceLabels are the human-readable names parallel to Sources (see
	// SourceVolume.Label) — empty (or shorter than Sources) when no
	// enrichment took place; the chart in the browser then falls back to
	// the IP address.
	SourceLabels []string
	Days         []time.Time
	Cells        [][]HeatmapCell
}
