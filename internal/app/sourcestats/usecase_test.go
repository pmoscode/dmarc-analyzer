package sourcestats_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

var errTest = errors.New("testfehler")

type fakeSourcesRepository struct {
	page sources.Page
	err  error
	got  sources.Query
}

func (f *fakeSourcesRepository) Query(_ context.Context, q sources.Query) (sources.Page, error) {
	f.got = q
	if f.err != nil {
		return sources.Page{}, f.err
	}
	return f.page, nil
}

type fakeEnricher struct {
	byIP  map[string]sources.Enrichment
	calls []string
}

func (f *fakeEnricher) Enrich(_ context.Context, ip report.SourceIP) sources.Enrichment {
	f.calls = append(f.calls, ip.String())
	return f.byIP[ip.String()]
}

func mustSourceIP(t *testing.T, s string) report.SourceIP {
	t.Helper()
	ip, err := report.NewSourceIP(s)
	require.NoError(t, err)
	return ip
}

func TestUseCase_List_EnrichesEveryRow(t *testing.T) {
	t.Parallel()

	ip1 := mustSourceIP(t, "203.0.113.1")
	ip2 := mustSourceIP(t, "203.0.113.2")

	repo := &fakeSourcesRepository{page: sources.Page{
		Stats: []sources.Stat{{SourceIP: ip1, TotalCount: 10}, {SourceIP: ip2, TotalCount: 5}},
	}}
	enricher := &fakeEnricher{byIP: map[string]sources.Enrichment{
		"203.0.113.1": {Hostname: "mail.google.com", Service: "Google Workspace"},
	}}
	uc := &sourcestats.UseCase{Sources: repo, Enricher: enricher}

	got, err := uc.List(context.Background(), sources.Query{})
	require.NoError(t, err)
	require.Len(t, got.Stats, 2)

	require.Equal(t, "mail.google.com", got.Stats[0].Enrichment.Hostname)
	require.Equal(t, "Google Workspace", got.Stats[0].Enrichment.Service)
	require.Empty(t, got.Stats[1].Enrichment.Hostname, "quelle ohne bekanntes PTR bleibt leer, kein Fehler")

	require.ElementsMatch(t, []string{"203.0.113.1", "203.0.113.2"}, enricher.calls)
}

func TestUseCase_List_ForwardsQueryToRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeSourcesRepository{}
	uc := &sourcestats.UseCase{Sources: repo, Enricher: &fakeEnricher{}}

	q := sources.Query{Domain: "example.com", Limit: 25, Cursor: "abc"}
	_, err := uc.List(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, q, repo.got)
}

func TestUseCase_List_RepositoryError_IsForwarded(t *testing.T) {
	t.Parallel()

	uc := &sourcestats.UseCase{Sources: &fakeSourcesRepository{err: errTest}, Enricher: &fakeEnricher{}}

	_, err := uc.List(context.Background(), sources.Query{})
	require.ErrorIs(t, err, errTest)
}

func TestUseCase_List_EmptyPage_NoEnrichmentCalls(t *testing.T) {
	t.Parallel()

	enricher := &fakeEnricher{}
	uc := &sourcestats.UseCase{Sources: &fakeSourcesRepository{}, Enricher: enricher}

	got, err := uc.List(context.Background(), sources.Query{})
	require.NoError(t, err)
	require.Empty(t, got.Stats)
	require.Empty(t, enricher.calls)
}
