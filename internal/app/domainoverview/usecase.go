// Package domainoverview orchestriert die Domains-Ansicht: aggregierte
// Statistik je veröffentlichter Policy-Domain (Ergänzung zur
// Sendequellen-Ansicht, internal/app/sourcestats — dort nach Quell-IP,
// hier nach Domain).
package domainoverview

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

// UseCase orchestriert Abfragen aggregierter Domains. Bewusst dünn (siehe
// internal/app/queryreports) — keine Anreicherung nötig wie bei
// sourcestats.UseCase, eine Domain hat keine PTR/Dienst-Erkennung.
type UseCase struct {
	Domains domainstats.Repository
}

// List liefert eine Seite aggregierter Domains für q.
func (uc *UseCase) List(ctx context.Context, q domainstats.Query) (domainstats.Page, error) {
	page, err := uc.Domains.Query(ctx, q)
	if err != nil {
		return domainstats.Page{}, fmt.Errorf("domains konnten nicht geladen werden: %w", err)
	}
	return page, nil
}
