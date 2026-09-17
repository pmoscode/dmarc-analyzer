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

// ErrCredentialStoreLocked wird von einem sperrbaren CredentialStore
// zurückgegeben (aktuell: internal/infra/keyring.LockableFileStore),
// solange die Master-Passphrase noch nicht eingegeben wurde
// (MIGRATIONSPLAN.md Erweiterung 9.4). Die Web-Oberfläche fängt diesen
// Fehler an jeder Stelle ab, die den Store benutzt, und leitet auf
// /entsperren um, statt einen technischen Fehler zu zeigen. Der
// OS-Schlüsselbund-Adapter kennt diesen Zustand nicht (liefert ihn nie) —
// dort ist nach dem Programmstart nichts zu entsperren.
var ErrCredentialStoreLocked = errors.New("schlüsselspeicher ist noch gesperrt — master-passphrase erforderlich")

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
//
// Vertrag für Store-Implementierungen: die übergebenen Bytes (secret.
// Expose()) dürfen nur synchron innerhalb des Aufrufs gelesen werden —
// nie eine Referenz auf das zugrunde liegende Slice über den Aufruf
// hinaus behalten. Aufrufer dürfen das übergebene Secret direkt danach
// mit Zero() überschreiben (IMPLEMENTIERUNG.md Abschnitt 9: "Secret wird
// erst unmittelbar vor dem Login gelesen und danach mit Zero()
// überschrieben"); ein Store, der die Bytes nicht sofort in eine eigene
// Kopie verwandelt (String-Konversion, Verschlüsselung, …), würde durch
// ein späteres Zero() nachträglich selbst geleert. Die echten Adapter
// (OSStore, FileStore) erfüllen das bereits von selbst; ein Test-Fake muss
// es sich bewusst machen (account.NewSecret(secret.Expose()) kopiert).
type CredentialStore interface {
	Store(accountID AccountID, secret Secret) error
	Retrieve(accountID AccountID) (Secret, error)
	Delete(accountID AccountID) error
}
