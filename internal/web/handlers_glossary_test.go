package web

import (
	"html"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/web/glossary"
)

func TestHandleGlossary_RendersAllTermsWithAnchors(t *testing.T) {
	srv, _ := newTestServer(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/glossar")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)

	for _, term := range glossary.Terms {
		require.Contains(t, got, `id="`+glossary.Slug(term.Name)+`"`, term.Name)
		require.Contains(t, got, html.EscapeString(term.Name), term.Name)
		require.Contains(t, got, html.EscapeString(term.Definition), term.Name)
	}
}

func TestHandleDashboard_TilesLinkToGlossary(t *testing.T) {
	srv, _ := newTestServer(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `href="/glossar#`+glossary.Slug("DMARC-Pass-Rate")+`"`)
	require.Contains(t, html, `href="/glossar#`+glossary.Slug("Nachrichtenvolumen pro Tag")+`"`)
	require.Contains(t, html, `href="/glossar#`+glossary.Slug("Top-Sendequellen")+`"`)
	require.Contains(t, html, `href="/glossar#`+glossary.Slug("Verteilung nach Disposition")+`"`)
	require.Contains(t, html, `href="/glossar#`+glossary.Slug("Sendequelle × Tag (Pass-Rate)")+`"`)
}
