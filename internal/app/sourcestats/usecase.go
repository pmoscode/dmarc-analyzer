// Package sourcestats orchestrates the sending sources view: aggregated
// statistics per source IP, enriched with PTR hostname and recognized
// service (IMPLEMENTIERUNG.md section 10.1 "sending sources").
//
// The enrichment (network I/O, see domain/sources.Enricher) deliberately
// lives here and not in the repository adapter: SQL aggregation and DNS
// resolution are two fundamentally different kinds of I/O, mixing them
// into one adapter would blur testability and responsibility.
package sourcestats

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// UseCase orchestrates queries for aggregated sending sources.
type UseCase struct {
	Sources  sources.Repository
	Enricher sources.Enricher
}

// List returns a page of aggregated sending sources for q, each row
// already enriched with Enrichment (hostname/service).
func (uc *UseCase) List(ctx context.Context, q sources.Query) (sources.Page, error) {
	page, err := uc.Sources.Query(ctx, q)
	if err != nil {
		return sources.Page{}, fmt.Errorf("failed to load sending sources: %w", err)
	}

	for i := range page.Stats {
		page.Stats[i].Enrichment = uc.Enricher.Enrich(ctx, page.Stats[i].SourceIP)
	}

	return page, nil
}
