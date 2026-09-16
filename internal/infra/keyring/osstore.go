package keyring

import (
	"errors"
	"fmt"

	zkeyring "github.com/zalando/go-keyring"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// serviceName identifiziert die Anwendung gegenüber dem OS-Schlüsselbund
// (macOS Keychain, Windows Credential Manager, Linux Secret Service).
// Festgelegt in UMSETZUNGSPLAN.md Abschnitt 3.4 — nachträglich nur mit
// Migrationsaufwand änderbar, da bereits gespeicherte Einträge sonst nicht
// mehr gefunden werden.
const serviceName = "de.pmoscode.dmarc-analyzer"

// OSStore implementiert account.CredentialStore gegen den
// Betriebssystem-Schlüsselbund.
type OSStore struct{}

var _ account.CredentialStore = OSStore{}

// NewOSStore erzeugt einen einsatzbereiten OSStore.
func NewOSStore() OSStore {
	return OSStore{}
}

// Store speichert secret. Ein bereits vorhandener Eintrag für dieselbe
// AccountID wird überschrieben (Verhalten des OS-Schlüsselbunds).
func (OSStore) Store(accountID account.AccountID, secret account.Secret) error {
	if err := zkeyring.Set(serviceName, string(accountID), string(secret.Expose())); err != nil {
		return fmt.Errorf("geheimnis für %q konnte nicht im schlüsselbund gespeichert werden: %w", accountID, err)
	}
	return nil
}

// Retrieve liest das gespeicherte Secret. Liefert
// account.ErrCredentialNotFound, wenn keines existiert.
func (OSStore) Retrieve(accountID account.AccountID) (account.Secret, error) {
	value, err := zkeyring.Get(serviceName, string(accountID))
	if errors.Is(err, zkeyring.ErrNotFound) {
		return account.Secret{}, account.ErrCredentialNotFound
	}
	if err != nil {
		return account.Secret{}, fmt.Errorf("geheimnis für %q konnte nicht aus dem schlüsselbund gelesen werden: %w", accountID, err)
	}
	return account.NewSecretFromString(value), nil
}

// Delete entfernt den Eintrag. Kein Fehler, wenn keiner existierte —
// Delete ist idempotent (IMPLEMENTIERUNG.md Abschnitt 9: "Löschen heißt
// löschen", eine bereits gelöschte Ressource erneut zu löschen ist kein
// Fehlerfall für den Aufrufer).
func (OSStore) Delete(accountID account.AccountID) error {
	err := zkeyring.Delete(serviceName, string(accountID))
	if err != nil && !errors.Is(err, zkeyring.ErrNotFound) {
		return fmt.Errorf("geheimnis für %q konnte nicht aus dem schlüsselbund gelöscht werden: %w", accountID, err)
	}
	return nil
}
