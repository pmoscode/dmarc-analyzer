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

	// /berichte, /quellen und /glossar haben inzwischen echten Inhalt
	// (eigene Tests in handlers_reports_test.go/handlers_sources_test.go/
	// handlers_glossary_test.go) — hier nur der verbleibende echte
	// Platzhalter.
	resp := httpGet(t, client, "http://"+srv.Addr()+"/einstellungen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "<title>Einstellungen")
	// Die Navigation muss auf allen fünf Seiten erscheinen (dasselbe
	// layout.html), auch auf dem noch inhaltslosen Platzhalter.
	require.Contains(t, html, `href="/berichte"`)
	require.Contains(t, html, `href="/quellen"`)
	require.Contains(t, html, `href="/glossar"`)
	require.Contains(t, html, `href="/einstellungen"`)
}

func TestPlaceholderPages_MarkCurrentNavItemActive(t *testing.T) {
	srv := newTestServer(t)
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
