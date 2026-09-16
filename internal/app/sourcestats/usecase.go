// Package sourcestats orchestriert die Sendequellen-Ansicht: aggregierte
// Statistik je Quell-IP, angereichert mit PTR-Hostname und erkanntem
// Dienst (IMPLEMENTIERUNG.md Abschnitt 10.1 "Sendequellen").
//
// Die Anreicherung (Netzwerk-I/O, siehe domain/sources.Enricher) läuft
// bewusst hier und nicht im Repository-Adapter: SQL-Aggregation und
// DNS-Auflösung sind zwei grundverschiedene I/O-Arten, deren Vermischung
// in einem Adapter Testbarkeit und Zuständigkeit verwässern würde.
package sourcestats

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// UseCase orchestriert Abfragen aggregierter Sendequellen.
type UseCase struct {
	Sources  sources.Repository
	Enricher sources.Enricher
}

// List liefert eine Seite aggregierter Sendequellen für q, jede Zeile
// bereits mit Enrichment (Hostname/Dienst) angereichert.
func (uc *UseCase) List(ctx context.Context, q sources.Query) (sources.Page, error) {
	page, err := uc.Sources.Query(ctx, q)
	if err != nil {
		return sources.Page{}, fmt.Errorf("sendequellen konnten nicht geladen werden: %w", err)
	}

	for i := range page.Stats {
		page.Stats[i].Enrichment = uc.Enricher.Enrich(ctx, page.Stats[i].SourceIP)
	}

	return page, nil
}
