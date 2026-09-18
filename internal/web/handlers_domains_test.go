package web

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/domainoverview"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
)

func newTestServerWithDomains(t *testing.T, repo *fakeDomainsRepository) *Server {
	t.Helper()

	deps := Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{}},
		Domains:    &domainoverview.UseCase{Domains: repo},
	}
	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv
}

func TestHandleDomains_RendersRows(t *testing.T) {
	repo := &fakeDomainsRepository{page: domainstats.Page{Stats: []domainstats.Stat{
		{Domain: mustDomainName("example.com"), TotalCount: 42, PassRate: 0.75, ReportCount: 3, DistinctSources: 2},
	}}}
	srv := newTestServerWithDomains(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "example.com")
	require.Contains(t, html, "42")
	require.Contains(t, html, "75.0 %")
	require.Contains(t, html, ">3<")
	require.Contains(t, html, ">2<")
	require.Contains(t, html, "/berichte?")
	require.Contains(t, html, "domain=example.com")
}

func TestHandleDomains_EmptyResult_ShowsEmptyState(t *testing.T) {
	srv := newTestServerWithDomains(t, &fakeDomainsRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Keine Domains")
}

func TestHandleDomains_FilterParamsReachRepository(t *testing.T) {
	repo := &fakeDomainsRepository{}
	srv := newTestServerWithDomains(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains?zeitraum=7&domain=example.com&sortierung=domain")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.Equal(t, domainstats.SortByDomain, repo.lastQuery.SortField)
	require.WithinDuration(t, repo.lastQuery.Period.Begin.AddDate(0, 0, 7), repo.lastQuery.Period.End, 0)
}

func TestHandleDomains_UnknownSort_FallsBackToVolume(t *testing.T) {
	repo := &fakeDomainsRepository{}
	srv := newTestServerWithDomains(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains?sortierung=unsinn")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, domainstats.SortByVolume, repo.lastQuery.SortField)
}

func TestHandleDomains_QueryError_Returns500(t *testing.T) {
	repo := &fakeDomainsRepository{queryErr: errTest}
	srv := newTestServerWithDomains(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleDomainsPage_ReturnsFragmentWithNextCursor(t *testing.T) {
	repo := &fakeDomainsRepository{page: domainstats.Page{
		Stats:      []domainstats.Stat{{Domain: mustDomainName("example.com"), TotalCount: 5, PassRate: 1}},
		NextCursor: "next-cursor",
	}}
	srv := newTestServerWithDomains(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains/seite?cursor=abc")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "abc", repo.lastQuery.Cursor)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	require.Contains(t, html, "example.com")
	require.Contains(t, html, "cursor=next-cursor")
	require.NotContains(t, html, "<html")
}

func TestHandleDomainsPage_NoMorePages_OmitsLoadMoreButton(t *testing.T) {
	srv := newTestServerWithDomains(t, &fakeDomainsRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/domains/seite")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "Weitere laden")
}
