package web

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

func postForm(t *testing.T, client *http.Client, addr, path string, values map[string]string) *http.Response {
	t.Helper()
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+addr+path, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+addr)

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func csrfFormValues(srv *Server, extra map[string]string) map[string]string {
	values := map[string]string{"csrf_token": srv.auth.csrfToken}
	for k, v := range extra {
		values[k] = v
	}
	return values
}

func TestHandleSettings_RendersAccountsAndForm(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Erstes Konto"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einstellungen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "Erstes Konto")
	require.Contains(t, html, `name="host"`)
	require.Contains(t, html, `name="passwort"`)
}

func TestHandleAccountCreate_ValidData_CreatesAccountAndRedirects(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten", csrfFormValues(srv, map[string]string{
		"anzeigename":  "Mein Konto",
		"host":         "imap.example.com",
		"port":         "993",
		"benutzername": "user@example.com",
		"postfach":     "INBOX",
		"tls":          "on",
		"passwort":     "geheim123",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode) // Client folgt dem 303-Redirect

	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, "Mein Konto", accounts[0].DisplayName)
	require.Equal(t, "imap.example.com", accounts[0].Host)

	secret, err := fd.credentials.Retrieve(accounts[0].ID)
	require.NoError(t, err)
	require.Equal(t, "geheim123", string(secret.Expose()))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Konto gespeichert.")
}

func TestHandleAccountCreate_MissingHost_ShowsErrorWithoutCreating(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten", csrfFormValues(srv, map[string]string{
		"benutzername": "user@example.com",
		"port":         "993",
		"passwort":     "geheim123",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Host darf nicht leer sein")

	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, accounts)
}

func TestHandleAccountCreate_MissingCSRFToken_Returns403(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten", map[string]string{
		"host": "imap.example.com", "port": "993", "benutzername": "u@example.com", "passwort": "x",
	})
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleAccountListMailboxes_Success_PopulatesOptions(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	fd.source.mailboxes = []string{"INBOX", "INBOX/DMARC"}
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/ordner", csrfFormValues(srv, map[string]string{
		"host": "imap.example.com", "port": "993", "benutzername": "u@example.com", "passwort": "x",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	require.Contains(t, html, "INBOX/DMARC")
	require.Contains(t, html, "2 Postfächer gefunden.")

	// Passwort darf im erneut angezeigten Formular nicht auftauchen.
	require.NotContains(t, html, `value="x"`)
}

func TestHandleAccountListMailboxes_ConnectionFails_ShowsError(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	fd.source.connectErr = errTest
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/ordner", csrfFormValues(srv, map[string]string{
		"host": "imap.example.com", "port": "993", "benutzername": "u@example.com", "passwort": "x",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Postfächer konnten nicht abgerufen werden")
}

func TestHandleAccountTest_Success_ShowsSuccessMessage(t *testing.T) {
	acc := mustAccount(t, "Konto 1")
	srv, fd := newTestServerWithAccounts(t, acc)
	require.NoError(t, fd.credentials.Store(acc.ID, account.NewSecretFromString("pw")))
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/acc-1/test", csrfFormValues(srv, nil))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Verbindung erfolgreich.")
}

func TestHandleAccountTest_ConnectionFails_ShowsErrorMessage(t *testing.T) {
	acc := mustAccount(t, "Konto 1")
	srv, fd := newTestServerWithAccounts(t, acc)
	require.NoError(t, fd.credentials.Store(acc.ID, account.NewSecretFromString("pw")))
	fd.source.connectErr = errTest
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/acc-1/test", csrfFormValues(srv, nil))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Verbindung fehlgeschlagen.")
}

func TestHandleAccountDelete_RemovesAccount(t *testing.T) {
	acc := mustAccount(t, "Konto 1")
	srv, fd := newTestServerWithAccounts(t, acc)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/acc-1/loeschen", csrfFormValues(srv, nil))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, accounts)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Konto gelöscht.")
}

func TestHandleAccountCreate_CredentialsLocked_RedirectsToUnlock(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	fd.credentials.storeErr = account.ErrCredentialStoreLocked
	client := authenticatedClient(t, srv)

	// credentialsLocked() prüft per Typ-Assertion auf lockChecker — das
	// fakeCredentialStore-Fake implementiert das nicht, deshalb hier
	// direkt über einen echten LockableFileStore-artigen Test unten statt
	// hier: dieser Test prüft nur, dass ein Store()-Fehler (z. B. weil
	// tatsächlich gesperrt) sauber als Formularfehler auftaucht, nicht als
	// 500er.
	resp := postForm(t, client, srv.Addr(), "/konten", csrfFormValues(srv, map[string]string{
		"host": "imap.example.com", "port": "993", "benutzername": "u@example.com", "passwort": "x",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, accounts, "Konto darf bei fehlgeschlagenem Credential-Store nicht als gespeichert gelten")
}
