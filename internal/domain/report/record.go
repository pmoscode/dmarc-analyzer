package report

import "fmt"

// Record ist eine ausgewertete Sendequelle innerhalb eines Reports:
// wie viele Nachrichten von dieser IP kamen, mit welchem Ergebnis. Records
// existieren nur im Kontext ihres AggregateReport (siehe report.go).
type Record struct {
	SourceIP    SourceIP
	Count       int
	Evaluated   PolicyEvaluation
	Identifiers Identifiers
	Auth        AuthResults
}

// NewRecord erzwingt Count > 0 (IMPLEMENTIERUNG.md Abschnitt 6.2).
func NewRecord(sourceIP SourceIP, count int, evaluated PolicyEvaluation, identifiers Identifiers, auth AuthResults) (Record, error) {
	if count <= 0 {
		return Record{}, fmt.Errorf("count muss größer als 0 sein, war %d", count)
	}
	if !sourceIP.IsValid() {
		return Record{}, fmt.Errorf("record ohne gültige quell-ip ist ungültig")
	}

	return Record{
		SourceIP:    sourceIP,
		Count:       count,
		Evaluated:   evaluated,
		Identifiers: identifiers,
		Auth:        auth,
	}, nil
}
