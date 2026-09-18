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
	return newTestServerWithSourcesAndIMAPHost(t, repo, enricher, "")
}

func newTestServerWithSourcesAndIMAPHost(t *testing.T, repo *fakeSourcesRepository, enricher *fakeEnricher, imapHost string) *Server {
	t.Helper()

	if enricher == nil {
		enricher = &fakeEnricher{}
	}

	deps := Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{}},
		Sources:    &sourcestats.UseCase{Sources: repo, Enricher: enricher},
		IMAPHost:   imapHost,
	}
	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
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

func TestClassifySource(t *testing.T) {
	cases := []struct {
		name      string
		dkim, spf float64
		wantLabel string
		wantTone  string
	}{
		{"dkim und spf bestehen", 1.0, 1.0, "Autorisiert (DKIM & SPF)", "good"},
		{"nur dkim besteht: weiterleitung", 1.0, 0.0, "Autorisiert (vermutlich Weiterleitung)", "good"},
		{"dkim knapp über schwelle, spf mittelmäßig", 0.95, 0.6, "Autorisiert (vermutlich Weiterleitung)", "good"},
		{"dkim besteht überwiegend nicht", 0.2, 0.0, "Nicht bestätigt — prüfen", "critical"},
		{"uneindeutig dazwischen", 0.7, 0.3, "Teilweise bestätigt — prüfen", "warning"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, tone := classifySource(domainsources.Stat{DKIMPassRate: tc.dkim, SPFPassRate: tc.spf})
			require.Equal(t, tc.wantLabel, label)
			require.Equal(t, tc.wantTone, tone)
		})
	}
}

func TestRegistrableDomain(t *testing.T) {
	cases := map[string]string{
		"dd33832.kasserver.com":    "kasserver.com",
		"w0119144.kasserver.com":   "kasserver.com",
		"mail-sor-f41.google.com.": "google.com",
		"localhost":                "localhost",
		"":                         "",
	}
	for in, want := range cases {
		require.Equal(t, want, registrableDomain(in), in)
	}
}

func TestHandleSources_SameHosterAsIMAP_ShowsBadge(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{Stats: []domainsources.Stat{
		{SourceIP: mustSourceIP("203.0.113.1"), TotalCount: 10, PassRate: 1, DKIMPassRate: 1, SPFPassRate: 1},
	}}}
	enricher := &fakeEnricher{enrichment: domainsources.Enrichment{Hostname: "dd33832.kasserver.com"}}
	srv := newTestServerWithSourcesAndIMAPHost(t, repo, enricher, "w0119144.kasserver.com")
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "gleicher Hoster wie IMAP")
	require.Contains(t, html, "Autorisiert (DKIM &amp; SPF)")
}

func TestHandleSources_DifferentHosterThanIMAP_NoBadge(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{Stats: []domainsources.Stat{
		{SourceIP: mustSourceIP("203.0.113.1"), TotalCount: 10, PassRate: 1, DKIMPassRate: 1, SPFPassRate: 0},
	}}}
	enricher := &fakeEnricher{enrichment: domainsources.Enrichment{Hostname: "fritz.lanhost.de"}}
	srv := newTestServerWithSourcesAndIMAPHost(t, repo, enricher, "w0119144.kasserver.com")
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/quellen")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.NotContains(t, html, "gleicher Hoster wie IMAP")
	require.Contains(t, html, "Autorisiert (vermutlich Weiterleitung)")
}
