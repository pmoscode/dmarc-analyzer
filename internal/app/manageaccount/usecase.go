// Package manageaccount stellt Diagnosefunktionen für das eine, per ENV
// konfigurierte Mail-Konto bereit (siehe internal/infra/envconfig) —
// Anlegen/Löschen von Konten gibt es seit dem Umstieg auf ENV-Konfiguration
// nicht mehr, nur noch "Verbindung testen" und "Konto anzeigen".
package manageaccount

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// UseCase orchestriert die Diagnose des konfigurierten Kontos.
type UseCase struct {
	Accounts account.Repository
	// Secret ist das aus ENV geladene IMAP-Passwort — gilt für die gesamte
	// Prozesslaufzeit, es gibt kein Ändern/Speichern mehr zur Laufzeit.
	Secret account.Secret
	// NewSource liefert je Verbindungstest eine frische, unverbundene
	// MessageSource.
	NewSource func() sync.MessageSource
}

// TestConnectionByID lädt das konfigurierte Konto und testet die
// Verbindung — für den "Verbindung testen"-Knopf auf der Status-Seite.
func (uc *UseCase) TestConnectionByID(ctx context.Context, id account.AccountID) error {
	acc, err := uc.Accounts.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("konto %q konnte nicht geladen werden: %w", id, err)
	}

	source := uc.NewSource()
	defer func() { _ = source.Close() }()

	if err := source.Connect(ctx, *acc, uc.Secret); err != nil {
		return fmt.Errorf("verbindungstest fehlgeschlagen: %w", err)
	}
	return nil
}

// List liefert alle konfigurierten Konten — aktuell immer genau eines
// (siehe internal/infra/envconfig), für die Status-Seite.
func (uc *UseCase) List(ctx context.Context) ([]account.MailAccount, error) {
	accounts, err := uc.Accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("konten konnten nicht geladen werden: %w", err)
	}
	return accounts, nil
}
