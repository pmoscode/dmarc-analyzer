package sync

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// State ist der Fortschritt des inkrementellen Syncs je Konto und Postfach
// (IMPLEMENTIERUNG.md Abschnitt 7.1). Wird nach jeder erfolgreich
// verarbeiteten Nachricht fortgeschrieben, damit ein Abbruch höchstens eine
// Nachricht erneut kostet.
type State struct {
	AccountID account.AccountID
	Mailbox   string
	// UIDValidity identifiziert die "Generation" des Postfachs. Ändert sie
	// sich zwischen zwei Syncs, ist ein vollständiger Rescan nötig — der
	// UNIQUE-Index auf Reports verhindert dabei Duplikate.
	UIDValidity uint32
	LastUID     uint32
	LastSyncAt  time.Time
}

// StateRepository ist der Port zur Persistenz von State, ein Eintrag pro
// (AccountID, Mailbox). Implementiert in AP 4 gegen SQLite
// (internal/infra/sqlite.SyncStateRepository) — bewusst erst hier
// definiert, wo der Sync-Use-Case (syncreports) ihn zum ersten Mal
// tatsächlich braucht, nicht schon in AP 1/3 spekulativ.
type StateRepository interface {
	// Load liefert den Nullwert State{AccountID: accountID, Mailbox:
	// mailbox} ohne Fehler, wenn noch kein Fortschritt gespeichert ist —
	// ein erster Sync ist kein Fehlerfall.
	Load(ctx context.Context, accountID account.AccountID, mailbox string) (State, error)
	Save(ctx context.Context, state State) error
}
