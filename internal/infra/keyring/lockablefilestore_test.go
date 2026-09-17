package keyring_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
)

func TestLockableFileStore_StartsLocked(t *testing.T) {
	t.Parallel()

	store := keyring.NewLockableFileStore(t.TempDir())
	require.True(t, store.Locked())
}

func TestLockableFileStore_OperationsReturnErrCredentialStoreLockedBeforeUnlock(t *testing.T) {
	t.Parallel()

	store := keyring.NewLockableFileStore(t.TempDir())

	_, err := store.Retrieve("account-1")
	require.ErrorIs(t, err, account.ErrCredentialStoreLocked)

	err = store.Store("account-1", account.NewSecretFromString("geheim"))
	require.ErrorIs(t, err, account.ErrCredentialStoreLocked)

	err = store.Delete("account-1")
	require.ErrorIs(t, err, account.ErrCredentialStoreLocked)
}

func TestLockableFileStore_Unlock_AllowsOperations(t *testing.T) {
	t.Parallel()

	store := keyring.NewLockableFileStore(t.TempDir())
	require.NoError(t, store.Unlock(account.NewSecretFromString("master-passphrase")))
	require.False(t, store.Locked())

	secret := account.NewSecretFromString("app-passwort")
	require.NoError(t, store.Store("account-1", secret))

	got, err := store.Retrieve("account-1")
	require.NoError(t, err)
	require.Equal(t, "app-passwort", string(got.Expose()))
}

func TestLockableFileStore_Lock_ReturnsToLockedState(t *testing.T) {
	t.Parallel()

	store := keyring.NewLockableFileStore(t.TempDir())
	require.NoError(t, store.Unlock(account.NewSecretFromString("master-passphrase")))
	require.False(t, store.Locked())

	store.Lock()
	require.True(t, store.Locked())

	_, err := store.Retrieve("account-1")
	require.ErrorIs(t, err, account.ErrCredentialStoreLocked)
}

func TestLockableFileStore_WrongPassphraseAfterUnlock_FailsOnRetrieve(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Erstes Unlock mit der "richtigen" Passphrase legt Salt + Secret an.
	store := keyring.NewLockableFileStore(dir)
	require.NoError(t, store.Unlock(account.NewSecretFromString("richtige-passphrase")))
	require.NoError(t, store.Store("account-1", account.NewSecretFromString("app-passwort")))
	store.Lock()

	// Zweites Unlock mit falscher Passphrase: Unlock() selbst schlägt
	// nicht fehl (siehe Doku), aber Retrieve() eines existierenden
	// Secrets scheitert an der Entschlüsselung.
	require.NoError(t, store.Unlock(account.NewSecretFromString("falsche-passphrase")))
	_, err := store.Retrieve("account-1")
	require.Error(t, err)
	require.False(t, errors.Is(err, account.ErrCredentialStoreLocked))
}
