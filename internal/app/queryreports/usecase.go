// Package queryreports orchestriert das Filtern, Sortieren und
// Gruppieren gespeicherter Reports sowie das Laden eines einzelnen
// Reports für die Detailansicht (IMPLEMENTIERUNG.md Abschnitt 10.1).
//
// Bewusst dünn: die eigentliche Filter-/Sortier-/Paginierungslogik steckt
// im report.Repository-Adapter (AP 2). Dieser Use Case ist trotzdem kein
// überflüssiger Umweg — er ist die Nahtstelle, an der internal/ui hängt,
// statt direkt an einem Infra-Adapter (AGENTS.md: "internal/ui/*: ruft
// ausschließlich Use Cases aus internal/app auf").
package queryreports

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// UseCase orchestriert Abfragen gespeicherter Reports.
type UseCase struct {
	Reports report.Repository
}

// List liefert eine Seite von Reports gemäß q — für die Berichtstabelle.
func (uc *UseCase) List(ctx context.Context, q report.Query) (report.Page, error) {
	return uc.Reports.Query(ctx, q)
}

// Get lädt einen Report vollständig, inklusive aller Records — für die
// Bericht-Detailansicht.
func (uc *UseCase) Get(ctx context.Context, id report.ReportID) (*report.AggregateReport, error) {
	return uc.Reports.FindByID(ctx, id)
}
