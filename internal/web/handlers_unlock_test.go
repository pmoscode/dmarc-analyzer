package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
)

// newTestServerWithLockableCredentials baut einen Server wie
// newTestServerWithAccounts, ersetzt Dependencies.Credentials aber durch
// einen echten (gesperrten) keyring.LockableFileStore statt des simplen
// fakeCredentialStore — für Tests von /entsperren, die einen echten
// Sperr-/Entsperr-Zyklus brauchen (Locked()/Unlock()/Lock()).
func newTestServerWithLockableCredentials(t *testing.T, accounts ...account.MailAccount) (*Server, *keyring.LockableFileStore) {
	t.Helper()
	srv, _ := newTestServerWithAccounts(t, accounts...)
	store := keyring.NewLockableFileStore(t.TempDir())
	srv.deps.Credentials = store
	return srv, store
}

func TestHandleUnlockForm_Locked_ShowsForm(t *testing.T) {
	srv, _ := newTestServerWithLockableCredentials(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/entsperren?weiter=/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	require.Contains(t, html, `name="passphrase"`)
	require.Contains(t, html, `value="/berichte"`)
}

func TestHandleUnlockForm_AlreadyUnlocked_RedirectsToNext(t *testing.T) {
	srv, store := newTestServerWithLockableCredentials(t, mustAccount(t, "Konto 1"))
	require.NoError(t, store.Unlock(account.NewSecretFromString("passphrase")))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/entsperren?weiter=/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Berichte")
}

func TestHandleUnlockSubmit_NoExistingAccount_AcceptsAnyPassphrase(t *testing.T) {
	srv, store := newTestServerWithLockableCredentials(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/entsperren", csrfFormValues(srv, map[string]string{
		"passphrase": "irgendeine-passphrase",
		"next":       "/einrichtung",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.False(t, store.Locked())
}

func TestHandleUnlockSubmit_WrongPassphraseWithExistingAccount_ShowsErrorAndRelocks(t *testing.T) {
	acc := mustAccount(t, "Konto 1")
	srv, store := newTestServerWithLockableCredentials(t, acc)

	// Konto-Secret mit der "richtigen" Passphrase anlegen.
	require.NoError(t, store.Unlock(account.NewSecretFromString("richtige-passphrase")))
	require.NoError(t, store.Store(acc.ID, account.NewSecretFromString("app-passwort")))
	store.Lock()

	client := authenticatedClient(t, srv)
	resp := postForm(t, client, srv.Addr(), "/entsperren", csrfFormValues(srv, map[string]string{
		"passphrase": "falsche-passphrase",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Falsche Master-Passphrase.")
	require.True(t, store.Locked(), "bei falscher Passphrase muss wieder gesperrt werden")
}

func TestHandleUnlockSubmit_CorrectPassphraseWithExistingAccount_Unlocks(t *testing.T) {
	acc := mustAccount(t, "Konto 1")
	srv, store := newTestServerWithLockableCredentials(t, acc)

	require.NoError(t, store.Unlock(account.NewSecretFromString("richtige-passphrase")))
	require.NoError(t, store.Store(acc.ID, account.NewSecretFromString("app-passwort")))
	store.Lock()

	client := authenticatedClient(t, srv)
	resp := postForm(t, client, srv.Addr(), "/entsperren", csrfFormValues(srv, map[string]string{
		"passphrase": "richtige-passphrase",
		"next":       "/einstellungen",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.False(t, store.Locked())

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Einstellungen")
}

func TestHandleUnlockSubmit_NextOutsideSite_FallsBackToDashboard(t *testing.T) {
	// Mit einem existierenden Konto bleibt "/" wirklich die Übersicht,
	// statt selbst auf /einrichtung umzuleiten (keine Konten wäre ein
	// zweiter, hier nicht interessierender Umleitungsgrund).
	srv, _ := newTestServerWithLockableCredentials(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/entsperren", csrfFormValues(srv, map[string]string{
		"passphrase": "x",
		"next":       "//evil.example.com",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://"+srv.Addr()+"/", resp.Request.URL.String())
}
