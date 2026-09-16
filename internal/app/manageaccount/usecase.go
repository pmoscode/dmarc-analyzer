// Package manageaccount orchestriert Anlegen, Testen und Löschen von
// Mail-Konten (IMPLEMENTIERUNG.md Abschnitt 6.4/9). Schließt insbesondere
// die in AP 3 bewusst zurückgestellte Hälfte "Kontolöschung entfernt
// Metadaten UND Keyring-Eintrag" (UMSETZUNGSPLAN.md AP 3).
package manageaccount

import (
	"context"
	"errors"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// UseCase orchestriert die Kontoverwaltung.
type UseCase struct {
	Accounts    account.Repository
	Credentials account.CredentialStore
	// NewSource liefert je Verbindungstest eine frische, unverbundene
	// MessageSource.
	NewSource func() sync.MessageSource
}

// Create legt ein Konto an: Metadaten und Zugangsdaten werden zusammen
// gespeichert. Schlägt das Speichern der Zugangsdaten fehl, werden die
// bereits gespeicherten Metadaten wieder entfernt (best effort) — ein
// Konto ohne zugehöriges Geheimnis wäre nutzlos und würde beim nächsten
// Sync nur verwirrend scheitern.
func (uc *UseCase) Create(ctx context.Context, acc *account.MailAccount, secret account.Secret) error {
	if err := uc.Accounts.Save(ctx, acc); err != nil {
		return fmt.Errorf("konto %q konnte nicht gespeichert werden: %w", acc.ID, err)
	}

	if err := uc.Credentials.Store(acc.ID, secret); err != nil {
		_ = uc.Accounts.Delete(ctx, acc.ID) // best effort, siehe Doku oben
		return fmt.Errorf("zugangsdaten für konto %q konnten nicht gespeichert werden: %w", acc.ID, err)
	}
	return nil
}

// TestConnection baut probeweise eine Verbindung auf (ohne etwas zu
// synchronisieren) — für den Verbindungstest beim Einrichten eines
// Kontos (IMPLEMENTIERUNG.md Abschnitt 10.1, Einstellungen).
func (uc *UseCase) TestConnection(ctx context.Context, acc account.MailAccount, secret account.Secret) error {
	source := uc.NewSource()
	defer func() { _ = source.Close() }()

	if err := source.Connect(ctx, acc, secret); err != nil {
		return fmt.Errorf("verbindungstest fehlgeschlagen: %w", err)
	}
	return nil
}

// TestConnectionByID lädt ein bereits gespeichertes Konto samt
// Zugangsdaten und testet die Verbindung — für "Verbindung testen" gegen
// ein existierendes Konto (im Unterschied zu TestConnection, das beim
// Einrichten eines noch ungespeicherten Kontos verwendet wird).
func (uc *UseCase) TestConnectionByID(ctx context.Context, id account.AccountID) error {
	acc, err := uc.Accounts.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("konto %q konnte nicht geladen werden: %w", id, err)
	}

	secret, err := uc.Credentials.Retrieve(id)
	if err != nil {
		return fmt.Errorf("zugangsdaten für konto %q konnten nicht geladen werden: %w", id, err)
	}
	defer secret.Zero()

	return uc.TestConnection(ctx, *acc, secret)
}

// Delete entfernt ein Konto vollständig: Metadaten UND Schlüsselbund-
// Eintrag (IMPLEMENTIERUNG.md Abschnitt 9, Regel "Löschen heißt löschen").
// Beide Schritte laufen unabhängig voneinander — schlägt einer fehl, wird
// der andere trotzdem versucht, damit kein Löschversuch auf halbem Weg
// steckenbleibt.
func (uc *UseCase) Delete(ctx context.Context, id account.AccountID) error {
	credErr := uc.Credentials.Delete(id)
	metaErr := uc.Accounts.Delete(ctx, id)

	if credErr != nil || metaErr != nil {
		return fmt.Errorf("konto %q konnte nicht vollständig gelöscht werden: %w",
			id, errors.Join(credErr, metaErr))
	}
	return nil
}

// List liefert alle konfigurierten Konten — für die Kontoübersicht.
func (uc *UseCase) List(ctx context.Context) ([]account.MailAccount, error) {
	accounts, err := uc.Accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("konten konnten nicht geladen werden: %w", err)
	}
	return accounts, nil
}
