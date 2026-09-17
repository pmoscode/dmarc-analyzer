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

	for _, path := range []string{"/", "/berichte", "/quellen", "/glossar", "/einstellungen"} {
		resp := httpGet(t, client, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()

		html := string(body)
		require.Contains(t, html, `href="/"`, path)
		require.Contains(t, html, `href="/berichte"`, path)
		require.Contains(t, html, `href="/quellen"`, path)
		require.Contains(t, html, `href="/glossar"`, path)
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

	for _, path := range []string{"/berichte", "/quellen", "/glossar", "/einstellungen", "/einrichtung", "/entsperren"} {
		resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
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
