// Package queryfailedrecords orchestriert die Fehlschläge-Ansicht:
// berichtsübergreifende Suche nach Records, bei denen DMARC nicht
// bestanden wurde (Ergänzung zur Berichte-Detailseite, die dieselbe
// Rohdaten-Aufbereitung für einen einzelnen Bericht zeigt).
package queryfailedrecords

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

// UseCase orchestriert Abfragen fehlgeschlagener Records. Bewusst dünn
// (siehe internal/app/queryreports/domainoverview) — Filter/Sortierung/
// Pagination stecken vollständig im Repository-Adapter.
type UseCase struct {
	Records failedrecords.Repository
}

// List liefert eine Seite fehlgeschlagener Records für q.
func (uc *UseCase) List(ctx context.Context, q failedrecords.Query) (failedrecords.Page, error) {
	page, err := uc.Records.Query(ctx, q)
	if err != nil {
		return failedrecords.Page{}, fmt.Errorf("fehlgeschlagene records konnten nicht geladen werden: %w", err)
	}
	return page, nil
}
