package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNav_AppearsOnEveryRealPage(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	for _, path := range []string{"/", "/berichte", "/quellen", "/domains", "/einstellungen"} {
		resp := httpGet(t, client, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()

		html := string(body)
		require.Contains(t, html, `href="/"`, path)
		require.Contains(t, html, `href="/berichte"`, path)
		require.Contains(t, html, `href="/quellen"`, path)
		require.Contains(t, html, `href="/domains"`, path)
		require.Contains(t, html, `href="/einstellungen"`, path)
	}
}

func TestNav_MarksCurrentPageActive(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einstellungen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `href="/einstellungen" class="active"`)
	require.NotContains(t, html, `href="/berichte" class="active"`)
}

func TestRealPages_RequireSession(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))

	// CheckRedirect stoppt vor dem Redirect-Ziel: /anmelden würde sonst
	// bis zum (nicht auflösbaren) Fake-Provider weiterverfolgt, siehe
	// testRedirectURL-Dokumentation in server_test.go.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for _, path := range []string{"/berichte", "/quellen", "/domains", "/einstellungen"} {
		resp := httpGet(t, client, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusSeeOther, resp.StatusCode, path)
		require.Equal(t, "/anmelden", resp.Header.Get("Location"), path)
		_ = resp.Body.Close()
	}
}

func TestHandleDashboard_MarksOverviewNavItemActive(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `href="/" class="active"`)
}
