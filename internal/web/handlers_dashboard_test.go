package web

import (
	"html"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/web/glossary"
)

func TestHandleDashboard_RendersFilterBarWithSelectedValues(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/?zeitraum=90&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `value="90" selected`)
	require.Contains(t, html, `value="example.com"`)
}

func TestHandleDashboard_RendersFilterResetLink(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/?zeitraum=90&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `<a href="/" class="filter-reset">Filter zurücksetzen</a>`)
}

func TestHandleDashboard_FilterParams_ReachRepository(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/?zeitraum=7&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastDailyVolumesQuery.Domain)
}

// TestHandleDashboard_GlossaryHints_ShowDefinitionInline verifies that
// the "?" hints carry the term explanation directly as an attribute
// (app.css shows it via a tooltip) instead of linking to a separate
// glossary page — there is no such page anymore, see
// TestGlossarRoute_NoLongerExists.
func TestHandleDashboard_GlossaryHints_ShowDefinitionInline(t *testing.T) {
	srv := newTestServerWithRepo(t, &fakeRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)

	require.NotContains(t, got, "/glossar")

	for _, term := range []string{"DMARC-Pass-Rate", "Alignment", "Quell-IP", "Nachrichtenvolumen pro Tag", "Top-Sendequellen", "Verteilung nach Disposition", "Sendequelle × Tag (Pass-Rate)"} {
		def, ok := glossary.ByName(term)
		require.True(t, ok, term)
		require.Contains(t, got, `data-tip="`+html.EscapeString(def.Definition)+`"`, term)
	}
}

func TestGlossarRoute_NoLongerExists(t *testing.T) {
	srv := newTestServerWithRepo(t, &fakeRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/glossar")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
