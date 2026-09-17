package web

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

func newTestServerWithReports(t *testing.T, repo report.Repository) *Server {
	t.Helper()

	deps := Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{}},
		Reports:    &queryreports.UseCase{Reports: repo},
	}
	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv
}

func mustAggregateReport(t *testing.T, id report.ReportID, org, domain string, begin time.Time) report.AggregateReport {
	t.Helper()
	period, err := report.NewDateRange(begin, begin.AddDate(0, 0, 1))
	require.NoError(t, err)
	dn, err := report.NewDomainName(domain)
	require.NoError(t, err)

	return report.AggregateReport{
		ID: id,
		Metadata: report.Metadata{
			OrgName:  org,
			Email:    "noc@" + domain,
			ReportID: "report-" + org,
			Range:    period,
		},
		Policy: report.PublishedPolicy{Domain: dn, Policy: report.PolicyReject, SubdomainPolicy: report.PolicyReject, DKIMAlignment: report.AlignmentRelaxed, SPFAlignment: report.AlignmentRelaxed, Percentage: 100},
	}
}

func TestHandleReports_RendersRowsWithLinks(t *testing.T) {
	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	r1 := mustAggregateReport(t, 1, "Google", "example.com", begin)
	repo := &fakeReportRepository{page: report.Page{Reports: []report.AggregateReport{r1}}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `href="/berichte/1"`)
	require.Contains(t, html, "Google")
	require.Contains(t, html, "example.com")
	require.Contains(t, html, "2026-09-01")
}

func TestHandleReports_EmptyResult_ShowsEmptyState(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Keine Berichte")
}

func TestHandleReports_FilterParamsReachRepository(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+
		"/berichte?domain=example.com&quelle=203.0.113.1&disposition=reject&gruppierung=domain&sortierung=org_name&sortrichtung=asc")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.Equal(t, "203.0.113.1", repo.lastQuery.SourceIP)
	require.Equal(t, report.DispositionReject, repo.lastQuery.Disposition)
	require.Equal(t, report.GroupByDomain, repo.lastQuery.GroupBy)
	require.Equal(t, report.SortByOrgName, repo.lastQuery.SortField)
	require.Equal(t, report.SortAscending, repo.lastQuery.SortDirection)
}

func TestHandleReports_UnknownSortAndGroup_FallBackToDefaults(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte?sortierung=unsinn&gruppierung=unsinn&sortrichtung=unsinn")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, report.SortByDateBegin, repo.lastQuery.SortField)
	require.Equal(t, report.SortDescending, repo.lastQuery.SortDirection)
	require.Equal(t, report.GroupByNone, repo.lastQuery.GroupBy)
}

func TestHandleReports_DrilldownVonBis_UsesAbsolutePeriod(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte?von=2026-09-01&bis=2026-09-02&quelle=203.0.113.1")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NotNil(t, repo.lastQuery.Period)
	require.Equal(t, "2026-09-01", repo.lastQuery.Period.Begin.Format("2006-01-02"))
	require.Equal(t, "2026-09-02", repo.lastQuery.Period.End.Format("2006-01-02"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "2026-09-01")
}

func TestHandleReports_QueryError_Returns500(t *testing.T) {
	repo := &fakeReportRepository{queryErr: errTest}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleReportsPage_ReturnsFragmentWithNextCursor(t *testing.T) {
	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	r1 := mustAggregateReport(t, 1, "Google", "example.com", begin)
	repo := &fakeReportRepository{page: report.Page{Reports: []report.AggregateReport{r1}, NextCursor: "next-cursor"}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte/seite?cursor=abc")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "abc", repo.lastQuery.Cursor)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	require.Contains(t, html, `href="/berichte/1"`)
	require.Contains(t, html, "cursor=next-cursor")
	// Das Fragment ist NICHT die volle Seite — kein <html>/<head>.
	require.NotContains(t, html, "<html")
}

func TestHandleReportsPage_NoMorePages_OmitsLoadMoreButton(t *testing.T) {
	repo := &fakeReportRepository{page: report.Page{NextCursor: ""}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte/seite")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "Weitere laden")
}

func TestHandleReportDetail_RendersMetadataPolicyAndRecords(t *testing.T) {
	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	full := mustAggregateReport(t, 42, "Google", "example.com", begin)
	ip := mustSourceIP("203.0.113.1")
	rec, err := report.NewRecord(ip, 10, report.PolicyEvaluation{Disposition: report.DispositionNone, DKIM: report.AuthResultPass, SPF: report.AuthResultFail}, report.Identifiers{}, report.AuthResults{})
	require.NoError(t, err)
	full.Records = []report.Record{rec}

	repo := &fakeReportRepository{byID: map[report.ReportID]*report.AggregateReport{42: &full}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte/42")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, "Google")
	require.Contains(t, html, "example.com")
	require.Contains(t, html, "report-Google")
	require.Contains(t, html, "203.0.113.1")
	require.Contains(t, html, "pass")
	require.Contains(t, html, "fail")
}

func TestHandleReportDetail_UnknownID_Returns500(t *testing.T) {
	repo := &fakeReportRepository{byID: map[report.ReportID]*report.AggregateReport{}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte/999")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleReportDetail_NonNumericID_Returns404(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/berichte/nicht-numerisch")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
