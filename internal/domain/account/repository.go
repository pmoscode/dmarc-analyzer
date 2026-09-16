package account

import (
	"context"
	"errors"
)

// ErrCredentialNotFound wird von CredentialStore.Retrieve zurückgegeben,
// wenn für die AccountID kein Geheimnis gespeichert ist. Ein gemeinsamer
// Sentinel-Fehler über alle CredentialStore-Implementierungen hinweg
// (OS-Schlüsselbund, Datei-Fallback) — Aufrufer prüfen mit errors.Is, ohne
// wissen zu müssen, welcher Adapter gerade aktiv ist.
var ErrCredentialNotFound = errors.New("kein geheimnis für diese account-id gespeichert")

// Repository ist der Port zur Persistenz der MailAccount-Metadaten
// (niemals Zugangsdaten — die liegen hinter CredentialStore). Implementiert
// in AP 4 gegen SQLite, zusammen mit der Kontoverwaltung
// (internal/app/manageaccount).
type Repository interface {
	Save(ctx context.Context, a *MailAccount) error
	FindByID(ctx context.Context, id AccountID) (*MailAccount, error)
	FindAll(ctx context.Context) ([]MailAccount, error)
	// Delete entfernt nur die Metadaten. Der zugehörige Schlüsselbund-
	// Eintrag wird separat über CredentialStore.Delete entfernt — die
	// Kontolöschung im Anwendungsfall (AP 4) ruft beides auf
	// (IMPLEMENTIERUNG.md Abschnitt 9, Regel "Löschen heißt löschen").
	Delete(ctx context.Context, id AccountID) error
}

// CredentialStore ist der Port zum sicheren Speichern von Zugangsdaten.
// Implementiert in AP 3 gegen den OS-Schlüsselbund
// (internal/infra/keyring), mit verschlüsseltem Dateispeicher als
// Linux-Fallback (IMPLEMENTIERUNG.md Abschnitt 9).
type CredentialStore interface {
	Store(accountID AccountID, secret Secret) error
	Retrieve(accountID AccountID) (Secret, error)
	Delete(accountID AccountID) error
}
