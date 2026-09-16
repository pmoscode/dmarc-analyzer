package report

import "errors"

// Sentinel-Fehler für die Invarianten aus NewAggregateReport — als
// eigene Variablen, damit Aufrufer sie mit errors.Is unterscheiden können
// (z. B. um sie in der UI unterschiedlich anzuzeigen).
var (
	errReportIDRequired  = errors.New("report ohne report_id ist ungültig")
	errDateRangeRequired = errors.New("report ohne gültigen zeitraum ist ungültig")
)
