package web

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/queryfailedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func newTestServerWithFailedRecords(t *testing.T, repo *fakeFailedRecordsRepository) *Server {
	t.Helper()

	deps := Dependencies{
		Statistics:    &statistics.UseCase{Repository: &fakeRepository{}},
		FailedRecords: &queryfailedrecords.UseCase{Records: repo},
	}
	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv
}

func TestHandleFailedRecords_RendersRows(t *testing.T) {
	repo := &fakeFailedRecordsRepository{page: failedrecords.Page{Records: []failedrecords.Record{
		{
			ReportID:     42,
			OrgName:      "Google",
			PolicyDomain: mustDomainName("example.com"),
			SourceIP:     mustSourceIP("198.51.100.1"),
			Count:        7,
			Disposition:  report.DispositionReject,
			DKIM:         report.AuthResultFail,
			SPF:          report.AuthResultFail,
		},
	}}}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "Google")
	require.Contains(t, html, "example.com")
	require.Contains(t, html, "198.51.100.1")
	require.Contains(t, html, "/berichte/42")
	require.Contains(t, html, "reject")
	require.Contains(t, html, "fail")
}

func TestHandleFailedRecords_RendersFilterResetLink(t *testing.T) {
	srv := newTestServerWithFailedRecords(t, &fakeFailedRecordsRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege?zeitraum=7&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `<a href="/fehlschlaege" class="filter-reset">Reset filter</a>`)
}

func TestHandleFailedRecords_EmptyResult_ShowsEmptyState(t *testing.T) {
	srv := newTestServerWithFailedRecords(t, &fakeFailedRecordsRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "No failed records")
}

func TestHandleFailedRecords_FilterParamsReachRepository(t *testing.T) {
	repo := &fakeFailedRecordsRepository{}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege?zeitraum=7&domain=example.com&quelle=198.51.100.1")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.Equal(t, "198.51.100.1", repo.lastQuery.SourceIP)
}

func TestHandleFailedRecords_UnknownSort_FallsBackToDate(t *testing.T) {
	repo := &fakeFailedRecordsRepository{}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege?sortierung=unsinn")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, failedrecords.SortByDate, repo.lastQuery.SortField)
}

func TestHandleFailedRecords_QueryError_Returns500(t *testing.T) {
	repo := &fakeFailedRecordsRepository{queryErr: errTest}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleFailedRecordsPage_ReturnsFragmentWithNextCursor(t *testing.T) {
	repo := &fakeFailedRecordsRepository{page: failedrecords.Page{
		Records:    []failedrecords.Record{{ReportID: 1, SourceIP: mustSourceIP("198.51.100.1"), PolicyDomain: mustDomainName("other.example")}},
		NextCursor: "next-cursor",
	}}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege/seite?cursor=abc")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Equal(t, "abc", repo.lastQuery.Cursor)
	require.Contains(t, html, "cursor=next-cursor")
	require.NotContains(t, html, "<html")
}

func TestHandleFailedRecordsPage_NoMorePages_OmitsLoadMoreButton(t *testing.T) {
	repo := &fakeFailedRecordsRepository{page: failedrecords.Page{
		Records: []failedrecords.Record{{ReportID: 1, SourceIP: mustSourceIP("198.51.100.1"), PolicyDomain: mustDomainName("example.com")}},
	}}
	srv := newTestServerWithFailedRecords(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/fehlschlaege/seite")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "Load more")
}
