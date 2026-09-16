package queryreports_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

type fakeReportRepository struct {
	queryResult report.Page
	queryErr    error
	byID        map[report.ReportID]*report.AggregateReport
	findErr     error
	lastQuery   report.Query
}

func (f *fakeReportRepository) Save(context.Context, *report.AggregateReport) error { return nil }
func (f *fakeReportRepository) Exists(context.Context, report.Key) (bool, error)    { return false, nil }

func (f *fakeReportRepository) FindByID(_ context.Context, id report.ReportID) (*report.AggregateReport, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.byID[id], nil
}

func (f *fakeReportRepository) Query(_ context.Context, q report.Query) (report.Page, error) {
	f.lastQuery = q
	return f.queryResult, f.queryErr
}

func TestList_ForwardsQueryAndResult(t *testing.T) {
	t.Parallel()

	want := report.Page{NextCursor: "next-page"}
	repo := &fakeReportRepository{queryResult: want}
	uc := &queryreports.UseCase{Reports: repo}

	q := report.Query{Domain: "example.com", Limit: 10}
	got, err := uc.List(context.Background(), q)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, q, repo.lastQuery, "Query muss unverändert durchgereicht werden")
}

func TestList_ForwardsError(t *testing.T) {
	t.Parallel()

	repo := &fakeReportRepository{queryErr: errTest}
	uc := &queryreports.UseCase{Reports: repo}

	_, err := uc.List(context.Background(), report.Query{})
	require.ErrorIs(t, err, errTest)
}

func TestGet_ReturnsReportByID(t *testing.T) {
	t.Parallel()

	r := &report.AggregateReport{ID: 42}
	repo := &fakeReportRepository{byID: map[report.ReportID]*report.AggregateReport{42: r}}
	uc := &queryreports.UseCase{Reports: repo}

	got, err := uc.Get(context.Background(), 42)
	require.NoError(t, err)
	require.Same(t, r, got)
}

func TestGet_ForwardsError(t *testing.T) {
	t.Parallel()

	repo := &fakeReportRepository{findErr: errTest}
	uc := &queryreports.UseCase{Reports: repo}

	_, err := uc.Get(context.Background(), 1)
	require.ErrorIs(t, err, errTest)
}
