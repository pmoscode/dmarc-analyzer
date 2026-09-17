package web

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

func newTestServerWithSources(t *testing.T, repo *fakeSourcesRepository, enricher *fakeEnricher) *Server {
	t.Helper()
	isolateConfigDir(t)

	if enricher == nil {
		enricher = &fakeEnricher{}
	}

	// Statistics wird gebraucht, weil die Anmeldung auf "/" weiterleitet
	// (siehe newTestServerWithReports für dieselbe Begründung).
	deps := Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{}},
		Sources:    &sourcestats.UseCase{Sources: repo, Enricher: enricher},
	}
	srv, err := New(deps, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	_, err = srv.Start(context.Background())
	require.NoError(t, err)
	return srv
}

func TestHandleSources_RendersRowsWithEnrichment(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{Stats: []domainsources.Stat{
		{SourceIP: mustSourceIP("203.0.113.1"), TotalCount: 42, PassRate: 0.75},
	}}}
	enricher := &fakeEnricher{enrichment: domainsources.Enrichment{Hostname: "mail.example.com", Service: "Google Workspace"}}
	srv := newTestServerWithSources(t, repo, enricher)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "203.0.113.1")
	require.Contains(t, html, "42")
	require.Contains(t, html, "75.0 %")
	require.Contains(t, html, "mail.example.com")
	require.Contains(t, html, "Google Workspace")
}

func TestHandleSources_NoEnrichment_ShowsPlaceholderValue(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{Stats: []domainsources.Stat{
		{SourceIP: mustSourceIP("203.0.113.9"), TotalCount: 1, PassRate: 0},
	}}}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), sourcesUnknownValue)
}

func TestHandleSources_EmptyResult_ShowsEmptyState(t *testing.T) {
	srv := newTestServerWithSources(t, &fakeSourcesRepository{}, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Keine Sendequellen")
}

func TestHandleSources_FilterParamsReachRepository(t *testing.T) {
	repo := &fakeSourcesRepository{}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen?zeitraum=7&domain=example.com&sortierung=source_ip")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.Equal(t, domainsources.SortByIP, repo.lastQuery.SortField)
	require.WithinDuration(t, repo.lastQuery.Period.Begin.AddDate(0, 0, 7), repo.lastQuery.Period.End, 0)
}

func TestHandleSources_UnknownSort_FallsBackToVolume(t *testing.T) {
	repo := &fakeSourcesRepository{}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen?sortierung=unsinn")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, domainsources.SortByVolume, repo.lastQuery.SortField)
}

func TestHandleSources_QueryError_Returns500(t *testing.T) {
	repo := &fakeSourcesRepository{queryErr: errTest}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleSourcesPage_ReturnsFragmentWithNextCursor(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{
		Stats:      []domainsources.Stat{{SourceIP: mustSourceIP("203.0.113.1"), TotalCount: 5, PassRate: 1}},
		NextCursor: "next-cursor",
	}}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen/seite?cursor=abc")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "abc", repo.lastQuery.Cursor)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	require.Contains(t, html, "203.0.113.1")
	require.Contains(t, html, "cursor=next-cursor")
	require.NotContains(t, html, "<html")
}

func TestHandleSourcesPage_NoMorePages_OmitsLoadMoreButton(t *testing.T) {
	srv := newTestServerWithSources(t, &fakeSourcesRepository{}, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen/seite")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "Weitere laden")
}
