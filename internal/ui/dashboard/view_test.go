package dashboard

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

func testPeriod(t *testing.T) report.DateRange {
	t.Helper()
	dr, err := report.NewDateRange(time.Now().Add(-24*time.Hour), time.Now())
	require.NoError(t, err)
	return dr
}

func TestView_SetFilter_NoMessages_ShowsEmptyState(t *testing.T) {
	repo := &fakeRepository{stats: analysis.Statistics{TotalMessages: 0}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := NewView(&statistics.UseCase{Repository: repo}, &fakeChartRenderer{}, w)
	v.runBackground = func(f func()) { f() }
	w.SetContent(v)

	v.SetFilter(testPeriod(t), "")

	require.NotNil(t, uitest.FindLabel(v, i18n.DashboardEmptyTitle))
}

func TestView_SetFilter_WithMessages_ShowsTilesAndCharts(t *testing.T) {
	repo := &fakeRepository{stats: analysis.Statistics{
		TotalMessages: 100, PassRate: 0.9, DKIMAlignmentRate: 0.8, SPFAlignmentRate: 0.7, DistinctSources: 3,
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := NewView(&statistics.UseCase{Repository: repo}, &fakeChartRenderer{}, w)
	v.runBackground = func(f func()) { f() }
	w.SetContent(v)

	v.SetFilter(testPeriod(t), "")

	require.Nil(t, uitest.FindLabel(v, i18n.DashboardEmptyTitle))
	require.NotNil(t, uitest.FindLabel(v, "100"))
	require.NotNil(t, uitest.FindLabel(v, "90.0%"))
	require.NotNil(t, uitest.FindLabel(v, "80.0%"))
	require.NotNil(t, uitest.FindLabel(v, "70.0%"))
	require.NotNil(t, uitest.FindLabel(v, "3"))

	require.NotNil(t, v.dailyPanel.image())
	require.NotNil(t, v.topPanel.image())
	require.NotNil(t, v.dispPanel.image())
	require.NotNil(t, v.heatPanel.image())
}

func TestView_SetFilter_ForwardsPeriodAndDomain(t *testing.T) {
	repo := &fakeRepository{stats: analysis.Statistics{TotalMessages: 1}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := NewView(&statistics.UseCase{Repository: repo}, &fakeChartRenderer{}, w)
	v.runBackground = func(f func()) { f() }
	w.SetContent(v)

	period := testPeriod(t)
	v.SetFilter(period, "example.com")

	require.Equal(t, "example.com", repo.lastQuery.Domain)
	require.True(t, repo.lastQuery.Period.Begin.Equal(period.Begin))
}

func TestView_SetFilter_RepositoryError_ShowsErrorDialogNoPanic(t *testing.T) {
	repo := &fakeRepository{computeErr: errTest}
	w := test.NewWindow(nil)
	defer w.Close()

	v := NewView(&statistics.UseCase{Repository: repo}, &fakeChartRenderer{}, w)
	v.runBackground = func(f func()) { f() }
	w.SetContent(v)

	require.NotPanics(t, func() { v.SetFilter(testPeriod(t), "") })
}

func TestView_SetFilter_ChartRenderError_ShowsErrorDialogNoPanic(t *testing.T) {
	repo := &fakeRepository{stats: analysis.Statistics{TotalMessages: 5}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := NewView(&statistics.UseCase{Repository: repo}, &fakeChartRenderer{err: errTest}, w)
	v.runBackground = func(f func()) { f() }
	w.SetContent(v)

	require.NotPanics(t, func() { v.SetFilter(testPeriod(t), "") })
}
