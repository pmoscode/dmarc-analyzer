// Package sync enthält den Sync-Fortschritt (State) sowie den Port
// ReportParser, über den Rohanhänge zu Domänenobjekten werden.
//
// Der Port MessageSource (Zugriff auf ein Postfach) ist absichtlich noch
// nicht hier definiert: er hängt von account.MailAccount ab, das erst in
// AP 3 (Mail & Sicherheit) entsteht. Ihn jetzt schon mit einem Platzhalter-
// Kontotyp zu definieren, würde eine Abhängigkeit vortäuschen, die es noch
// nicht gibt — siehe UMSETZUNGSPLAN.md AP 1.
package sync

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// RawAttachment ist ein unverarbeiteter Anhang aus einer Quelle: Rohbytes
// plus die Metadaten, die zur Formaterkennung nötig sind.
type RawAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// ReportParser wandelt einen Rohanhang in Domänenobjekte. Über Supports()
// wird die passende Implementierung gewählt — so kommen RUF und TLS-RPT
// später additiv hinzu, ohne bestehenden Code zu ändern (Open/Closed,
// IMPLEMENTIERUNG.md Abschnitt 6.4).
type ReportParser interface {
	Supports(attachment RawAttachment) bool
	Parse(ctx context.Context, attachment RawAttachment) (*report.AggregateReport, error)
}
