package sync

import "time"

// State ist der Fortschritt des inkrementellen Syncs je Konto und Postfach
// (IMPLEMENTIERUNG.md Abschnitt 7.1). Wird nach jeder erfolgreich
// verarbeiteten Nachricht fortgeschrieben, damit ein Abbruch höchstens eine
// Nachricht erneut kostet.
type State struct {
	AccountID string
	Mailbox   string
	// UIDValidity identifiziert die "Generation" des Postfachs. Ändert sie
	// sich zwischen zwei Syncs, ist ein vollständiger Rescan nötig — der
	// UNIQUE-Index auf Reports verhindert dabei Duplikate.
	UIDValidity uint32
	LastUID     uint32
	LastSyncAt  time.Time
}
