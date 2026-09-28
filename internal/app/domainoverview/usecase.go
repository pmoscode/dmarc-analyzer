// Package domainoverview orchestrates the domains view: aggregated
// statistics per published policy domain (a complement to the sending
// sources view, internal/app/sourcestats — by source IP there, by domain
// here).
package domainoverview

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

// UseCase orchestrates queries for aggregated domains. Deliberately thin
// (see internal/app/queryreports) — no enrichment needed like in
// sourcestats.UseCase, a domain has no PTR/service recognition.
type UseCase struct {
	Domains domainstats.Repository
}

// List returns a page of aggregated domains for q.
func (uc *UseCase) List(ctx context.Context, q domainstats.Query) (domainstats.Page, error) {
	page, err := uc.Domains.Query(ctx, q)
	if err != nil {
		return domainstats.Page{}, fmt.Errorf("failed to load domains: %w", err)
	}
	return page, nil
}
