// Package statistics orchestrates the dashboard metrics
// (IMPLEMENTIERUNG.md section 10.2), including comparison to the
// previous period ("change vs. the previous period (trend arrow)") —
// that belongs in the application layer, not in the repository port: the
// previous period is a derived second query, not a persistence detail.
package statistics

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// UseCase orchestrates Statistics queries.
type UseCase struct {
	Repository analysis.Repository
	// Enricher enriches top sending sources and heatmap rows with
	// PTR hostname/recognized service (see SourceVolume.Label) —
	// optional: nil skips the enrichment (e.g. in tests), the dashboard
	// chart then keeps showing just the IP address.
	Enricher sources.Enricher
}

// Comparison sets the Statistics for a period against the Statistics of
// the immediately preceding period of equal length.
type Comparison struct {
	Current  analysis.Statistics
	Previous analysis.Statistics
	// PassRateTrend is Current.PassRate - Previous.PassRate, in
	// percentage points (can be negative). 0 if the previous period
	// contained no messages (Previous.TotalMessages == 0) — a comparison
	// against 0 messages would be misleading, not "0% improvement".
	PassRateTrend float64
	// HasPreviousPeriodData reports whether the previous period contained
	// any messages at all — otherwise the UI is better off showing "no
	// comparison possible" instead of a trend arrow based on 0/0.
	HasPreviousPeriodData bool
}

// ComputeWithTrend computes Statistics for q.Period as well as the
// immediately preceding period of equal length.
func (uc *UseCase) ComputeWithTrend(ctx context.Context, q analysis.Query) (Comparison, error) {
	current, err := uc.Repository.Compute(ctx, q)
	if err != nil {
		return Comparison{}, fmt.Errorf("failed to compute metrics for the period: %w", err)
	}

	previousQuery, err := previousPeriodQuery(q)
	if err != nil {
		return Comparison{}, err
	}

	previous, err := uc.Repository.Compute(ctx, previousQuery)
	if err != nil {
		return Comparison{}, fmt.Errorf("failed to compute metrics for the previous period: %w", err)
	}

	comparison := Comparison{Current: current, Previous: previous}
	if previous.TotalMessages > 0 {
		comparison.HasPreviousPeriodData = true
		comparison.PassRateTrend = current.PassRate - previous.PassRate
	}
	return comparison, nil
}

// topSourcesLimit is the number of sending sources for the bar chart and
// the heatmap (IMPLEMENTIERUNG.md section 10.3: "top 10 sending
// sources").
const topSourcesLimit = 10

// Dashboard bundles all data needed for the overview into one call:
// metrics including trend, time series, top sending sources and heatmap
// (IMPLEMENTIERUNG.md section 10.2/10.3) — one view, one load, instead of
// internal/ui/dashboard having to coordinate four use case methods
// individually.
type Dashboard struct {
	Comparison   Comparison
	DailyVolumes []analysis.DailyVolume
	TopSources   []analysis.SourceVolume
	Heatmap      analysis.Heatmap
}

// Dashboard loads all overview data for q.
func (uc *UseCase) Dashboard(ctx context.Context, q analysis.Query) (Dashboard, error) {
	comparison, err := uc.ComputeWithTrend(ctx, q)
	if err != nil {
		return Dashboard{}, err
	}

	dailyVolumes, err := uc.Repository.DailyVolumes(ctx, q)
	if err != nil {
		return Dashboard{}, fmt.Errorf("failed to compute time series: %w", err)
	}

	topSources, err := uc.Repository.TopSources(ctx, q, topSourcesLimit)
	if err != nil {
		return Dashboard{}, fmt.Errorf("failed to compute top sending sources: %w", err)
	}

	heatmap, err := uc.Repository.Heatmap(ctx, q, topSourcesLimit)
	if err != nil {
		return Dashboard{}, fmt.Errorf("failed to compute heatmap: %w", err)
	}

	uc.enrichSources(ctx, topSources, &heatmap)

	return Dashboard{
		Comparison:   comparison,
		DailyVolumes: dailyVolumes,
		TopSources:   topSources,
		Heatmap:      heatmap,
	}, nil
}

// enrichSources fills SourceVolume.Label (top sending sources chart) and
// Heatmap.SourceLabels (heatmap rows) via the optional Enricher — the
// same PTR/service enrichment as in the sending sources view
// (app/sourcestats.UseCase.List), here additionally for the dashboard.
// uc.Enricher == nil (e.g. in tests) simply skips this — the charts then
// keep showing the IP address.
func (uc *UseCase) enrichSources(ctx context.Context, topSources []analysis.SourceVolume, heatmap *analysis.Heatmap) {
	if uc.Enricher == nil {
		return
	}

	labels := make(map[string]string, len(topSources))
	for i := range topSources {
		label := sourceLabel(uc.Enricher.Enrich(ctx, topSources[i].SourceIP))
		topSources[i].Label = label
		labels[topSources[i].SourceIP.String()] = label
	}

	heatmap.SourceLabels = make([]string, len(heatmap.Sources))
	for i, ip := range heatmap.Sources {
		if label, ok := labels[ip.String()]; ok {
			heatmap.SourceLabels[i] = label
			continue
		}
		heatmap.SourceLabels[i] = sourceLabel(uc.Enricher.Enrich(ctx, ip))
	}
}

// sourceLabel prioritizes the recognized service over the plain PTR
// hostname — "Google Workspace" is more informative than
// "mail-sor-f41.google.com".
func sourceLabel(e sources.Enrichment) string {
	if e.Service != "" {
		return e.Service
	}
	return e.Hostname
}

// previousPeriodQuery returns the same query, but with a period of equal
// length immediately before q.Period.
func previousPeriodQuery(q analysis.Query) (analysis.Query, error) {
	duration := q.Period.End.Sub(q.Period.Begin)

	previousPeriod, err := report.NewDateRange(q.Period.Begin.Add(-duration), q.Period.Begin)
	if err != nil {
		return analysis.Query{}, fmt.Errorf("failed to compute previous period: %w", err)
	}

	return analysis.Query{Period: previousPeriod, Domain: q.Domain}, nil
}
