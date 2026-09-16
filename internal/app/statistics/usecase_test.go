package statistics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

var errTest = errors.New("testfehler")

type fakeStatsRepository struct {
	byPeriod map[time.Time]analysis.Statistics // keyed by Period.Begin
	err      error
	calls    []analysis.Query
}

func (f *fakeStatsRepository) Compute(_ context.Context, q analysis.Query) (analysis.Statistics, error) {
	f.calls = append(f.calls, q)
	if f.err != nil {
		return analysis.Statistics{}, f.err
	}
	return f.byPeriod[q.Period.Begin], nil
}

func period(t *testing.T, begin, end time.Time) report.DateRange {
	t.Helper()
	dr, err := report.NewDateRange(begin, end)
	require.NoError(t, err)
	return dr
}

func TestComputeWithTrend_CalculatesPreviousPeriodOfSameLength(t *testing.T) {
	t.Parallel()

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC) // 7 Tage

	repo := &fakeStatsRepository{}
	uc := &statistics.UseCase{Repository: repo}

	_, err := uc.ComputeWithTrend(context.Background(), analysis.Query{Period: period(t, begin, end)})
	require.NoError(t, err)
	require.Len(t, repo.calls, 2)

	require.True(t, repo.calls[0].Period.Begin.Equal(begin))
	require.True(t, repo.calls[0].Period.End.Equal(end))

	wantPrevBegin := begin.Add(-7 * 24 * time.Hour)
	require.True(t, repo.calls[1].Period.Begin.Equal(wantPrevBegin),
		"Vorperiode muss unmittelbar davor liegen und gleich lang sein")
	require.True(t, repo.calls[1].Period.End.Equal(begin))
}

func TestComputeWithTrend_CalculatesPassRateTrend(t *testing.T) {
	t.Parallel()

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	prevBegin := begin.Add(-7 * 24 * time.Hour)

	repo := &fakeStatsRepository{byPeriod: map[time.Time]analysis.Statistics{
		begin:     {TotalMessages: 100, PassRate: 0.95},
		prevBegin: {TotalMessages: 100, PassRate: 0.80},
	}}
	uc := &statistics.UseCase{Repository: repo}

	got, err := uc.ComputeWithTrend(context.Background(), analysis.Query{Period: period(t, begin, end)})
	require.NoError(t, err)
	require.True(t, got.HasPreviousPeriodData)
	require.InDelta(t, 0.15, got.PassRateTrend, 0.0001)
}

func TestComputeWithTrend_NoPreviousPeriodData_TrendIsZeroAndFlagged(t *testing.T) {
	t.Parallel()

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	repo := &fakeStatsRepository{byPeriod: map[time.Time]analysis.Statistics{
		begin: {TotalMessages: 50, PassRate: 0.9},
		// Vorperiode absichtlich nicht im Fixture → TotalMessages 0.
	}}
	uc := &statistics.UseCase{Repository: repo}

	got, err := uc.ComputeWithTrend(context.Background(), analysis.Query{Period: period(t, begin, end)})
	require.NoError(t, err)
	require.False(t, got.HasPreviousPeriodData)
	require.Zero(t, got.PassRateTrend)
}

func TestComputeWithTrend_CurrentPeriodError_IsForwarded(t *testing.T) {
	t.Parallel()

	repo := &fakeStatsRepository{err: errTest}
	uc := &statistics.UseCase{Repository: repo}

	_, err := uc.ComputeWithTrend(context.Background(), analysis.Query{
		Period: period(t, time.Now(), time.Now().Add(time.Hour)),
	})
	require.ErrorIs(t, err, errTest)
}

func TestComputeWithTrend_ForwardsDomainFilterToPreviousPeriod(t *testing.T) {
	t.Parallel()

	begin := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	repo := &fakeStatsRepository{}
	uc := &statistics.UseCase{Repository: repo}

	_, err := uc.ComputeWithTrend(context.Background(), analysis.Query{
		Period: period(t, begin, end), Domain: "example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "example.com", repo.calls[0].Domain)
	require.Equal(t, "example.com", repo.calls[1].Domain)
}

// erroringOnSecondCallRepository liefert beim ersten Compute-Aufruf
// (aktuelle Periode) ein Ergebnis, beim zweiten (Vorperiode) einen Fehler.
type erroringOnSecondCallRepository struct {
	calls int
}

func (r *erroringOnSecondCallRepository) Compute(context.Context, analysis.Query) (analysis.Statistics, error) {
	r.calls++
	if r.calls == 2 {
		return analysis.Statistics{}, errTest
	}
	return analysis.Statistics{TotalMessages: 10}, nil
}

func TestComputeWithTrend_PreviousPeriodError_IsForwarded(t *testing.T) {
	t.Parallel()

	uc := &statistics.UseCase{Repository: &erroringOnSecondCallRepository{}}

	_, err := uc.ComputeWithTrend(context.Background(), analysis.Query{
		Period: period(t, time.Now(), time.Now().Add(time.Hour)),
	})
	require.ErrorIs(t, err, errTest)
}
