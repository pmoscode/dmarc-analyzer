// Package queryfailedrecords orchestrates the failures view: a
// cross-report search for records where DMARC failed (a complement to
// the report detail page, which shows the same raw-data view for a
// single report).
package queryfailedrecords

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
)

// UseCase orchestrates queries for failed records. Deliberately thin
// (see internal/app/queryreports/domainoverview) — filtering/sorting/
// pagination live entirely in the repository adapter.
type UseCase struct {
	Records failedrecords.Repository
}

// List returns a page of failed records for q.
func (uc *UseCase) List(ctx context.Context, q failedrecords.Query) (failedrecords.Page, error) {
	page, err := uc.Records.Query(ctx, q)
	if err != nil {
		return failedrecords.Page{}, fmt.Errorf("failed to load failed records: %w", err)
	}
	return page, nil
}
