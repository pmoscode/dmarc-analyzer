package sources

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

type fakeSourcesRepository struct {
	pages     []domainsources.Page
	callIdx   int
	lastQuery domainsources.Query
}

func (f *fakeSourcesRepository) Query(_ context.Context, q domainsources.Query) (domainsources.Page, error) {
	f.lastQuery = q
	if f.callIdx >= len(f.pages) {
		return domainsources.Page{}, nil
	}
	p := f.pages[f.callIdx]
	f.callIdx++
	return p, nil
}

type fakeEnricher struct {
	byIP map[string]domainsources.Enrichment
}

func (f fakeEnricher) Enrich(_ context.Context, ip report.SourceIP) domainsources.Enrichment {
	return f.byIP[ip.String()]
}

func mustSourceIP(t *testing.T, s string) report.SourceIP {
	t.Helper()
	ip, err := report.NewSourceIP(s)
	require.NoError(t, err)
	return ip
}

func newSyncTestView(repo *fakeSourcesRepository, w fyne.Window) *View {
	return newSyncTestViewWithEnricher(repo, fakeEnricher{}, w)
}

func newSyncTestViewWithEnricher(repo *fakeSourcesRepository, enricher fakeEnricher, w fyne.Window) *View {
	v := NewView(&sourcestats.UseCase{Sources: repo, Enricher: enricher}, w)
	v.runBackground = func(f func()) { f() }
	return v
}

func TestView_Reload_NoSources_ShowsEmptyState(t *testing.T) {
	repo := &fakeSourcesRepository{pages: []domainsources.Page{{}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	require.NotNil(t, uitest.FindLabel(v, i18n.SourcesEmptyTitle))
}

func TestView_Reload_WithSources_ShowsFormattedRow(t *testing.T) {
	repo := &fakeSourcesRepository{pages: []domainsources.Page{{
		Stats: []domainsources.Stat{{SourceIP: mustSourceIP(t, "203.0.113.1"), TotalCount: 42, PassRate: 0.5}},
	}}}
	enricher := fakeEnricher{byIP: map[string]domainsources.Enrichment{
		"203.0.113.1": {Hostname: "mail.example.com", Service: "Beispieldienst"},
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestViewWithEnricher(repo, enricher, w)
	w.SetContent(v)
	v.Reload()

	require.Nil(t, uitest.FindLabel(v, i18n.SourcesEmptyTitle))
	require.NotNil(t, uitest.FindLabel(v, "203.0.113.1 — 42 Nachrichten, 50.0% Pass-Rate, mail.example.com (Beispieldienst)"))
}

func TestView_Reload_MissingEnrichment_ShowsPlaceholder(t *testing.T) {
	repo := &fakeSourcesRepository{pages: []domainsources.Page{{
		Stats: []domainsources.Stat{{SourceIP: mustSourceIP(t, "203.0.113.2"), TotalCount: 1, PassRate: 0}},
	}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	require.NotNil(t, uitest.FindLabel(v, "203.0.113.2 — 1 Nachrichten, 0.0% Pass-Rate, — (—)"))
}

func TestView_LoadMore_AppendsSecondPage(t *testing.T) {
	repo := &fakeSourcesRepository{pages: []domainsources.Page{
		{Stats: []domainsources.Stat{{SourceIP: mustSourceIP(t, "203.0.113.1"), TotalCount: 1}}, NextCursor: "cursor-1"},
		{Stats: []domainsources.Stat{{SourceIP: mustSourceIP(t, "203.0.113.2"), TotalCount: 1}}},
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()
	require.Len(t, v.data, 1)
	require.True(t, v.loadMore.Visible())

	v.loadMorePage()
	require.Len(t, v.data, 2)
	require.False(t, v.loadMore.Visible())
}

func TestView_SetFilter_ForwardsPeriodAndDomainToQuery(t *testing.T) {
	repo := &fakeSourcesRepository{pages: []domainsources.Page{{}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)

	dr, err := report.NewDateRange(time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)

	v.SetFilter(&dr, "example.com")

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.NotNil(t, repo.lastQuery.Period)
	require.True(t, repo.lastQuery.Period.Begin.Equal(dr.Begin))
}
