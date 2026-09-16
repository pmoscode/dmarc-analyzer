// Package statistics orchestriert die Dashboard-Kennzahlen
// (IMPLEMENTIERUNG.md Abschnitt 10.2), inklusive Vergleich zur Vorperiode
// ("Veränderung gegenüber der Vorperiode (Trendpfeil)") — das gehört in
// die Anwendungsschicht, nicht in den Repository-Port: Vorperiode ist eine
// abgeleitete zweite Abfrage, kein Persistenzdetail.
package statistics

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// UseCase orchestriert Statistics-Abfragen.
type UseCase struct {
	Repository analysis.Repository
	// Enricher reichert Top-Sendequellen und Heatmap-Zeilen mit
	// PTR-Hostname/erkanntem Dienst an (siehe SourceVolume.Label) —
	// optional: nil überspringt die Anreicherung (z. B. in Tests), das
	// Dashboard-Diagramm zeigt dann weiterhin nur die IP-Adresse.
	Enricher sources.Enricher
}

// Comparison stellt Statistics für einen Zeitraum den Statistics der
// unmittelbar davorliegenden, gleich langen Vorperiode gegenüber.
type Comparison struct {
	Current  analysis.Statistics
	Previous analysis.Statistics
	// PassRateTrend ist Current.PassRate - Previous.PassRate, in
	// Prozentpunkten (kann negativ sein). 0, wenn die Vorperiode keine
	// Nachrichten enthielt (Previous.TotalMessages == 0) — ein Vergleich
	// gegen 0 Nachrichten wäre irreführend, nicht "0 % Verbesserung".
	PassRateTrend float64
	// HasPreviousPeriodData meldet, ob die Vorperiode überhaupt
	// Nachrichten enthielt — die UI zeigt sonst besser "kein Vergleich
	// möglich" statt eines Trendpfeils auf Basis von 0/0.
	HasPreviousPeriodData bool
}

// ComputeWithTrend berechnet Statistics für q.Period sowie die
// unmittelbar davorliegende, gleich lange Vorperiode.
func (uc *UseCase) ComputeWithTrend(ctx context.Context, q analysis.Query) (Comparison, error) {
	current, err := uc.Repository.Compute(ctx, q)
	if err != nil {
		return Comparison{}, fmt.Errorf("kennzahlen für den zeitraum konnten nicht berechnet werden: %w", err)
	}

	previousQuery, err := previousPeriodQuery(q)
	if err != nil {
		return Comparison{}, err
	}

	previous, err := uc.Repository.Compute(ctx, previousQuery)
	if err != nil {
		return Comparison{}, fmt.Errorf("kennzahlen für die vorperiode konnten nicht berechnet werden: %w", err)
	}

	comparison := Comparison{Current: current, Previous: previous}
	if previous.TotalMessages > 0 {
		comparison.HasPreviousPeriodData = true
		comparison.PassRateTrend = current.PassRate - previous.PassRate
	}
	return comparison, nil
}

// topSourcesLimit ist die Anzahl Sendequellen für das Balkendiagramm und
// die Heatmap (IMPLEMENTIERUNG.md Abschnitt 10.3: "Top-10-Sendequellen").
const topSourcesLimit = 10

// Dashboard bündelt alle für die Übersicht benötigten Daten in einem
// Aufruf: Kennzahlen inklusive Trend, Zeitreihe, Top-Sendequellen und
// Heatmap (IMPLEMENTIERUNG.md Abschnitt 10.2/10.3) — eine Ansicht, ein
// Ladevorgang, statt dass internal/ui/dashboard vier Use-Case-Methoden
// einzeln koordinieren müsste.
type Dashboard struct {
	Comparison   Comparison
	DailyVolumes []analysis.DailyVolume
	TopSources   []analysis.SourceVolume
	Heatmap      analysis.Heatmap
}

// Dashboard lädt alle Übersichtsdaten für q.
func (uc *UseCase) Dashboard(ctx context.Context, q analysis.Query) (Dashboard, error) {
	comparison, err := uc.ComputeWithTrend(ctx, q)
	if err != nil {
		return Dashboard{}, err
	}

	dailyVolumes, err := uc.Repository.DailyVolumes(ctx, q)
	if err != nil {
		return Dashboard{}, fmt.Errorf("zeitreihe konnte nicht berechnet werden: %w", err)
	}

	topSources, err := uc.Repository.TopSources(ctx, q, topSourcesLimit)
	if err != nil {
		return Dashboard{}, fmt.Errorf("top-sendequellen konnten nicht berechnet werden: %w", err)
	}

	heatmap, err := uc.Repository.Heatmap(ctx, q, topSourcesLimit)
	if err != nil {
		return Dashboard{}, fmt.Errorf("heatmap konnte nicht berechnet werden: %w", err)
	}

	uc.enrichSources(ctx, topSources, &heatmap)

	return Dashboard{
		Comparison:   comparison,
		DailyVolumes: dailyVolumes,
		TopSources:   topSources,
		Heatmap:      heatmap,
	}, nil
}

// enrichSources füllt SourceVolume.Label (Top-Sendequellen-Diagramm) und
// Heatmap.SourceLabels (Heatmap-Zeilen) über den optionalen Enricher —
// dieselbe PTR-/Dienst-Anreicherung wie in der Sendequellen-Ansicht
// (app/sourcestats.UseCase.List), hier zusätzlich fürs Dashboard.
// uc.Enricher == nil (z. B. in Tests) überspringt das schlicht — die
// Diagramme zeigen dann weiterhin die IP-Adresse.
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

// sourceLabel priorisiert den erkannten Dienst vor dem reinen
// PTR-Hostname — "Google Workspace" ist aussagekräftiger als
// "mail-sor-f41.google.com".
func sourceLabel(e sources.Enrichment) string {
	if e.Service != "" {
		return e.Service
	}
	return e.Hostname
}

// previousPeriodQuery liefert dieselbe Query, aber mit einem Zeitraum
// gleicher Länge unmittelbar vor q.Period.
func previousPeriodQuery(q analysis.Query) (analysis.Query, error) {
	duration := q.Period.End.Sub(q.Period.Begin)

	previousPeriod, err := report.NewDateRange(q.Period.Begin.Add(-duration), q.Period.Begin)
	if err != nil {
		return analysis.Query{}, fmt.Errorf("vorperiode konnte nicht berechnet werden: %w", err)
	}

	return analysis.Query{Period: previousPeriod, Domain: q.Domain}, nil
}
