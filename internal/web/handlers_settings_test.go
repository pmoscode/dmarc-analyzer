package web

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func postForm(t *testing.T, client *http.Client, addr, path string, values map[string]string) *http.Response {
	t.Helper()
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	req := newRequest(t, http.MethodPost, "http://"+addr+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+testPublicHost)

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// csrfFormValues returns the form values with a valid CSRF token for
// client's session (see csrfTokenFor) — no form test in this package has
// needed other fields alongside it since the move to pure ENV
// configuration.
func csrfFormValues(t *testing.T, srv *Server, client *http.Client) map[string]string {
	t.Helper()
	return map[string]string{"csrf_token": csrfTokenFor(t, srv, client)}
}

func TestHandleSettings_RendersConfiguredAccountAndRuntimeSettings(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "First account"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einstellungen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "First account")
	require.Contains(t, html, "imap.example.com")
	require.Contains(t, html, "24 Monate")
	require.Contains(t, html, "alle 60 Minuten")
	require.NotContains(t, html, `name="passwort"`, "there is no account form anymore — credentials come from ENV")
}

func TestHandleSettings_NoAccountConfigured_ShowsEmptyState(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einstellungen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Kein Konto konfiguriert")
}

func TestHandleAccountTest_Success_ShowsSuccessMessage(t *testing.T) {
	acc := mustAccount(t, "Account 1")
	srv, _ := newTestServerWithAccounts(t, acc)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/"+string(acc.ID)+"/test", csrfFormValues(t, srv, client))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Verbindung erfolgreich.")
}

func TestHandleAccountTest_ConnectionFails_ShowsErrorMessage(t *testing.T) {
	acc := mustAccount(t, "Account 1")
	srv, fd := newTestServerWithAccounts(t, acc)
	fd.source.connectErr = errTest
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/"+string(acc.ID)+"/test", csrfFormValues(t, srv, client))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Verbindung fehlgeschlagen.")
}

func TestHandleAccountTest_MissingCSRFToken_Returns403(t *testing.T) {
	acc := mustAccount(t, "Account 1")
	srv, _ := newTestServerWithAccounts(t, acc)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/konten/"+string(acc.ID)+"/test", nil)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
