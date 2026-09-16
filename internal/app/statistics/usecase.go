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
)

// UseCase orchestriert Statistics-Abfragen.
type UseCase struct {
	Repository analysis.Repository
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
