package web

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

// pagedReportRepository returns reports spread across several pages (one
// report per page) — for the streaming CSV export test, which is meant
// to check that several pages are fetched one after another and all rows
// get written into a single CSV response.
type pagedReportRepository struct {
	fakeReportRepository
	pages     [][]report.AggregateReport
	fetched   int
	failAfter int // pages before an error is simulated (0: never)
}

func (p *pagedReportRepository) Query(_ context.Context, q report.Query) (report.Page, error) {
	p.lastQuery = q
	if p.failAfter > 0 && p.fetched >= p.failAfter {
		return report.Page{}, errTest
	}
	if p.fetched >= len(p.pages) {
		return report.Page{}, nil
	}
	reports := p.pages[p.fetched]
	p.fetched++
	next := ""
	if p.fetched < len(p.pages) {
		next = "cursor-" + string(rune('0'+p.fetched))
	}
	return report.Page{Reports: reports, NextCursor: next}, nil
}

func TestHandleExportReportsCSV_StreamsAllPages(t *testing.T) {
	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &pagedReportRepository{pages: [][]report.AggregateReport{
		{mustAggregateReport(t, 1, "Google", "example.com", begin)},
		{mustAggregateReport(t, 2, "Microsoft", "example.org", begin)},
		{mustAggregateReport(t, 3, "Yahoo", "example.net", begin)},
	}}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/export/berichte.csv")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Disposition"), "berichte.csv")

	rows, err := csv.NewReader(resp.Body).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 4) // header row + 3 reports across 3 pages
	require.Equal(t, "Google", rows[1][0])
	require.Equal(t, "Microsoft", rows[2][0])
	require.Equal(t, "Yahoo", rows[3][0])
}

func TestHandleExportReportsCSV_EmptyResult_OnlyHeader(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/export/berichte.csv")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the header row")
}

func TestHandleExportReportsCSV_UsesActiveFilter(t *testing.T) {
	repo := &fakeReportRepository{}
	srv := newTestServerWithReports(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/export/berichte.csv?domain=example.com&quelle=203.0.113.1")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.Equal(t, "203.0.113.1", repo.lastQuery.SourceIP)
}

func TestHandleExportSourcesCSV_StreamsRows(t *testing.T) {
	repo := &fakeSourcesRepository{page: domainsources.Page{Stats: []domainsources.Stat{
		{SourceIP: mustSourceIP("203.0.113.1"), TotalCount: 5, PassRate: 1},
		{SourceIP: mustSourceIP("203.0.113.2"), TotalCount: 3, PassRate: 0.5},
	}}}
	srv := newTestServerWithSources(t, repo, nil)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/export/quellen.csv")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Disposition"), "quellen.csv")

	rows, err := csv.NewReader(resp.Body).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 3) // Kopfzeile + 2 Quellen
	require.Equal(t, "203.0.113.1", rows[1][0])
	require.Equal(t, "203.0.113.2", rows[2][0])
}

func TestHandleExportReportsCSV_RequireSession(t *testing.T) {
	srv := newTestServerWithReports(t, &fakeReportRepository{})

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp := httpGet(t, client, "http://"+srv.Addr()+"/export/berichte.csv")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/anmelden", resp.Header.Get("Location"))
}
