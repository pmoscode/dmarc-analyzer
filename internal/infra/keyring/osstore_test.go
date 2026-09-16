package keyring_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
)

// skipIfOSKeyringUnavailable überspringt den Test, statt ihn scheitern zu
// lassen, wenn dieser Rechner keinen nutzbaren OS-Schlüsselbund hat (z. B.
// ein Linux-CI-Runner ohne laufenden Secret-Service-Dienst) — genau der
// Fall, für den FileStore als Fallback existiert und der in
// filestore_test.go deterministisch (ohne OS-Abhängigkeit) getestet wird.
func skipIfOSKeyringUnavailable(t *testing.T) {
	t.Helper()
	if !keyring.IsAvailable() {
		t.Skip("kein nutzbarer OS-Schlüsselbund auf diesem Rechner — siehe FileStore-Tests für den Fallback")
	}
}

func TestOSStore_StoreAndRetrieve_RoundTrip(t *testing.T) {
	skipIfOSKeyringUnavailable(t)

	store := keyring.NewOSStore()
	const id account.AccountID = "dmarc-analyzer-test-account-roundtrip"
	t.Cleanup(func() { _ = store.Delete(id) })

	require.NoError(t, store.Store(id, account.NewSecretFromString("test-app-passwort")))

	got, err := store.Retrieve(id)
	require.NoError(t, err)
	require.Equal(t, "test-app-passwort", string(got.Expose()))
}

func TestOSStore_Retrieve_NotFound(t *testing.T) {
	skipIfOSKeyringUnavailable(t)

	store := keyring.NewOSStore()
	_, err := store.Retrieve("dmarc-analyzer-test-account-nie-gespeichert")
	require.ErrorIs(t, err, account.ErrCredentialNotFound)
}

func TestOSStore_Delete_IsIdempotent(t *testing.T) {
	skipIfOSKeyringUnavailable(t)

	store := keyring.NewOSStore()
	const id account.AccountID = "dmarc-analyzer-test-account-delete"

	require.NoError(t, store.Store(id, account.NewSecretFromString("x")))
	require.NoError(t, store.Delete(id))
	require.NoError(t, store.Delete(id), "erneutes Löschen darf nicht scheitern")

	_, err := store.Retrieve(id)
	require.ErrorIs(t, err, account.ErrCredentialNotFound)
}

func TestIsAvailable_DoesNotLeaveProbeBehind(t *testing.T) {
	skipIfOSKeyringUnavailable(t)

	require.True(t, keyring.IsAvailable())

	// Ein zweiter Aufruf darf nicht an einem liegen gebliebenen
	// Kanarien-Eintrag aus dem ersten Aufruf scheitern.
	require.True(t, keyring.IsAvailable())
}
