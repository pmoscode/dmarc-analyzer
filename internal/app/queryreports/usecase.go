// Package queryreports orchestrates filtering, sorting and grouping
// saved reports, and loading a single report for the detail view
// (IMPLEMENTIERUNG.md section 10.1).
//
// Deliberately thin: the actual filter/sort/pagination logic lives in
// the report.Repository adapter (work package 2). This use case is still
// not a superfluous detour — it's the seam that internal/ui depends on,
// instead of on an infra adapter directly (AGENTS.md: "internal/ui/*:
// calls only use cases from internal/app").
package queryreports

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// UseCase orchestrates queries for saved reports.
type UseCase struct {
	Reports report.Repository
}

// List returns a page of reports according to q — for the reports table.
func (uc *UseCase) List(ctx context.Context, q report.Query) (report.Page, error) {
	return uc.Reports.Query(ctx, q)
}

// Get loads a report completely, including all records — for the report
// detail view.
func (uc *UseCase) Get(ctx context.Context, id report.ReportID) (*report.AggregateReport, error) {
	return uc.Reports.FindByID(ctx, id)
}
