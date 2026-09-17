package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlaceholderPages_RenderWithNavigationAndCorrectTitle(t *testing.T) {
	srv := newTestServer(t)
	client := authenticatedClient(t, srv)

	cases := []struct {
		path  string
		title string
	}{
		{"/berichte", "Berichte"},
		{"/quellen", "Sendequellen"},
		{"/glossar", "Glossar"},
		{"/einstellungen", "Einstellungen"},
	}

	for _, c := range cases {
		resp := httpGet(t, client, "http://"+srv.Addr()+c.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, c.path)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()

		html := string(body)
		require.Contains(t, html, "<title>"+c.title, c.path)
		// Die Navigation muss auf allen fünf Seiten erscheinen (dasselbe
		// layout.html), auch auf den noch inhaltslosen Platzhaltern.
		require.Contains(t, html, `href="/berichte"`, c.path)
		require.Contains(t, html, `href="/quellen"`, c.path)
		require.Contains(t, html, `href="/glossar"`, c.path)
		require.Contains(t, html, `href="/einstellungen"`, c.path)
	}
}

func TestPlaceholderPages_MarkCurrentNavItemActive(t *testing.T) {
	srv := newTestServer(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `href="/berichte" class="active"`)
	require.NotContains(t, html, `href="/quellen" class="active"`)
}

func TestPlaceholderPages_RequireSession(t *testing.T) {
	srv := newTestServer(t)

	for _, path := range []string{"/berichte", "/quellen", "/glossar", "/einstellungen"} {
		resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
		_ = resp.Body.Close()
	}
}

func TestHandleDashboard_MarksOverviewNavItemActive(t *testing.T) {
	srv := newTestServer(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `href="/" class="active"`)
}
