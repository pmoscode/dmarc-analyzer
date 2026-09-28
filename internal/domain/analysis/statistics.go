// Package analysis contains the metrics for the dashboard
// (IMPLEMENTIERUNG.md section 10.2) and the port through which they are
// computed.
package analysis

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// Query narrows down the Statistics computation.
type Query struct {
	Period report.DateRange
	// Domain optionally filters to a published policy domain. Empty means:
	// all domains.
	Domain string
}

// Statistics are the dashboard metrics for a period listed in
// IMPLEMENTIERUNG.md section 10.2. Counts are messages (sum of
// Record.Count), not records — a record with Count=50 is 50 messages from
// that source.
type Statistics struct {
	// TotalMessages is the total number of evaluated messages in the
	// period (sum of all Record.Count).
	TotalMessages int
	// PassRate is the share of messages that satisfy PassesDMARC()
	// (dkim=pass or spf=pass after alignment), in [0, 1]. 0 when
	// TotalMessages is 0.
	PassRate float64
	// SPFAlignmentRate and DKIMAlignmentRate are reported separately
	// (IMPLEMENTIERUNG.md section 10.2: "SPF alignment rate and DKIM
	// alignment rate reported separately").
	SPFAlignmentRate  float64
	DKIMAlignmentRate float64
	// DistinctSources is the number of distinct source IPs.
	DistinctSources int
	// VolumeByDisposition is the message count per disposition.
	VolumeByDisposition map[report.Disposition]int
}

// Repository is the port for computing Statistics. Implemented in work
// package 4 against SQLite (internal/infra/sqlite.StatisticsRepository)
// with SQL aggregations instead of Go-side computation over loaded
// records — the aggregation deliberately deferred in work package 2 (see
// UMSETZUNGSPLAN.md work package 2) lands here, tailored to exactly this
// use case instead of as a generic extension of report.Query.
type Repository interface {
	Compute(ctx context.Context, q Query) (Statistics, error)
	// DailyVolumes returns the message volume per day within q's period,
	// sorted ascending by day — for the time series (section 10.3).
	DailyVolumes(ctx context.Context, q Query) ([]DailyVolume, error)
	// TopSources returns the sending sources within q's period sorted
	// descending by volume, limited to limit entries.
	TopSources(ctx context.Context, q Query, limit int) ([]SourceVolume, error)
	// Heatmap returns the source-×-day matrix for the sourceLimit
	// highest-volume sources within q's period (same order as TopSources
	// with the same limit).
	Heatmap(ctx context.Context, q Query, sourceLimit int) (Heatmap, error)
}
