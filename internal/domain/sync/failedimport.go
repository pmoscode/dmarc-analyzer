package sync

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// FailedImport ist eine Nachricht (oder ein Anhang darin), die nicht
// verarbeitet werden konnte — landet in der Fehlerquarantäne statt den
// gesamten Sync-Lauf abzubrechen (IMPLEMENTIERUNG.md Abschnitt 7.2). In
// der UI einsehbar und gezielt wiederholbar (AP 5/7).
type FailedImport struct {
	AccountID  account.AccountID
	MessageUID uint32
	Filename   string
	Error      string
	// Raw ist der unverarbeitete Anhang bzw. die Nachricht — ermöglicht
	// ein erneutes Einlesen nach einer Parser-Korrektur, ohne das Postfach
	// erneut zu befragen.
	Raw        []byte
	OccurredAt time.Time
}

// FailedImportRepository ist der Port zur Fehlerquarantäne. Implementiert
// in AP 4 gegen SQLite (Tabelle failed_imports, Schema aus AP 2).
type FailedImportRepository interface {
	Record(ctx context.Context, f FailedImport) error
}
