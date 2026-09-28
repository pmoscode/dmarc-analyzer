package report

import "errors"

// Sentinel errors for the invariants in NewAggregateReport — kept as
// their own variables so callers can distinguish them with errors.Is
// (e.g. to display them differently in the UI).
var (
	errReportIDRequired  = errors.New("report without report_id is invalid")
	errDateRangeRequired = errors.New("report without a valid date range is invalid")
)
