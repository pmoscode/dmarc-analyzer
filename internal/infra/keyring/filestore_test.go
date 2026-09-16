package keyring_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
)

// newTestFileStore öffnet einen FileStore mit einer festen Test-
// Passphrase. Tests, die eine andere/falsche Passphrase brauchen, rufen
// keyring.NewFileStore direkt auf.
func newTestFileStore(t *testing.T) *keyring.FileStore {
	t.Helper()

	store, err := keyring.NewFileStore(t.TempDir(), account.NewSecretFromString("master-passphrase-123"))
	require.NoError(t, err)
	return store
}

func TestFileStore_StoreAndRetrieve_RoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestFileStore(t)
	secret := account.NewSecretFromString("app-passwort-des-postfachs")

	require.NoError(t, store.Store("account-1", secret))

	got, err := store.Retrieve("account-1")
	require.NoError(t, err)
	require.Equal(t, "app-passwort-des-postfachs", string(got.Expose()))
}

func TestFileStore_Retrieve_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestFileStore(t)

	_, err := store.Retrieve("nie-gespeichert")
	require.ErrorIs(t, err, account.ErrCredentialNotFound)
}

func TestFileStore_Delete_IsIdempotent(t *testing.T) {
	t.Parallel()

	store := newTestFileStore(t)
	require.NoError(t, store.Store("account-1", account.NewSecretFromString("x")))

	require.NoError(t, store.Delete("account-1"))
	require.NoError(t, store.Delete("account-1"), "erneutes Löschen eines bereits gelöschten Eintrags darf nicht scheitern")

	_, err := store.Retrieve("account-1")
	require.ErrorIs(t, err, account.ErrCredentialNotFound)
}

func TestFileStore_MultipleAccounts_AreIndependent(t *testing.T) {
	t.Parallel()

	store := newTestFileStore(t)
	require.NoError(t, store.Store("account-a", account.NewSecretFromString("geheimnis-a")))
	require.NoError(t, store.Store("account-b", account.NewSecretFromString("geheimnis-b")))

	a, err := store.Retrieve("account-a")
	require.NoError(t, err)
	require.Equal(t, "geheimnis-a", string(a.Expose()))

	b, err := store.Retrieve("account-b")
	require.NoError(t, err)
	require.Equal(t, "geheimnis-b", string(b.Expose()))

	require.NoError(t, store.Delete("account-a"))
	_, err = store.Retrieve("account-a")
	require.ErrorIs(t, err, account.ErrCredentialNotFound)

	// account-b darf vom Löschen von account-a unberührt bleiben.
	b, err = store.Retrieve("account-b")
	require.NoError(t, err)
	require.Equal(t, "geheimnis-b", string(b.Expose()))
}

func TestFileStore_Store_OverwritesExistingSecret(t *testing.T) {
	t.Parallel()

	store := newTestFileStore(t)
	require.NoError(t, store.Store("account-1", account.NewSecretFromString("alt")))
	require.NoError(t, store.Store("account-1", account.NewSecretFromString("neu")))

	got, err := store.Retrieve("account-1")
	require.NoError(t, err)
	require.Equal(t, "neu", string(got.Expose()))
}

func TestFileStore_WrongPassphrase_FailsToDecrypt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := keyring.NewFileStore(dir, account.NewSecretFromString("richtige-passphrase"))
	require.NoError(t, err)
	require.NoError(t, store.Store("account-1", account.NewSecretFromString("geheimnis")))

	wrongStore, err := keyring.NewFileStore(dir, account.NewSecretFromString("falsche-passphrase"))
	require.NoError(t, err, "NewFileStore selbst prüft die Passphrase nicht — erst Retrieve schlägt fehl")

	_, err = wrongStore.Retrieve("account-1")
	require.Error(t, err, "mit falscher Passphrase abgeleiteter Schlüssel darf nicht entschlüsseln können")
}

func TestFileStore_ReopeningWithSamePassphrase_Works(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, err := keyring.NewFileStore(dir, account.NewSecretFromString("passphrase"))
	require.NoError(t, err)
	require.NoError(t, first.Store("account-1", account.NewSecretFromString("geheimnis")))

	// Neuer Store gegen dasselbe Verzeichnis — simuliert einen
	// Programmneustart, bei dem der Salt aus der Datei wiederverwendet
	// wird (IMPLEMENTIERUNG.md Abschnitt 9: "Abfrage beim Programmstart").
	second, err := keyring.NewFileStore(dir, account.NewSecretFromString("passphrase"))
	require.NoError(t, err)

	got, err := second.Retrieve("account-1")
	require.NoError(t, err)
	require.Equal(t, "geheimnis", string(got.Expose()))
}

func TestFileStore_SecretsAreNotStoredAsPlaintextOnDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := keyring.NewFileStore(dir, account.NewSecretFromString("passphrase"))
	require.NoError(t, err)

	const plaintext = "eindeutig-erkennbares-app-passwort"
	require.NoError(t, store.Store("account-1", account.NewSecretFromString(plaintext)))

	data, err := os.ReadFile(filepath.Join(dir, "account-1.enc"))
	require.NoError(t, err)
	require.NotContains(t, string(data), plaintext)
}

func TestFileStore_SaltIsPersistedAndReused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := keyring.NewFileStore(dir, account.NewSecretFromString("passphrase"))
	require.NoError(t, err)

	saltPath := filepath.Join(dir, "salt")
	first, err := os.ReadFile(saltPath)
	require.NoError(t, err)
	require.Len(t, first, 16)

	_, err = keyring.NewFileStore(dir, account.NewSecretFromString("passphrase"))
	require.NoError(t, err)

	second, err := os.ReadFile(saltPath)
	require.NoError(t, err)
	require.Equal(t, first, second, "der Salt darf sich zwischen zwei Öffnungen nicht ändern")
}
