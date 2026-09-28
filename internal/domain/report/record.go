package report

import "fmt"

// Record is an evaluated sending source within a report: how many
// messages came from this IP, with what result. Records only exist in the
// context of their AggregateReport (see report.go).
type Record struct {
	SourceIP    SourceIP
	Count       int
	Evaluated   PolicyEvaluation
	Identifiers Identifiers
	Auth        AuthResults
}

// NewRecord enforces Count > 0 (IMPLEMENTIERUNG.md section 6.2).
func NewRecord(sourceIP SourceIP, count int, evaluated PolicyEvaluation, identifiers Identifiers, auth AuthResults) (Record, error) {
	if count <= 0 {
		return Record{}, fmt.Errorf("count must be greater than 0, was %d", count)
	}
	if !sourceIP.IsValid() {
		return Record{}, fmt.Errorf("record without a valid source IP is invalid")
	}

	return Record{
		SourceIP:    sourceIP,
		Count:       count,
		Evaluated:   evaluated,
		Identifiers: identifiers,
		Auth:        auth,
	}, nil
}
