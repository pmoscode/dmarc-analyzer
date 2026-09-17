package reports

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

type fakeReportRepository struct {
	pages     []report.Page
	callIdx   int
	byID      map[report.ReportID]*report.AggregateReport
	lastQuery report.Query
}

func (f *fakeReportRepository) Save(context.Context, *report.AggregateReport) error { return nil }
func (f *fakeReportRepository) Exists(context.Context, report.Key) (bool, error)    { return false, nil }

func (f *fakeReportRepository) FindByID(_ context.Context, id report.ReportID) (*report.AggregateReport, error) {
	if r, ok := f.byID[id]; ok {
		return r, nil
	}
	return nil, errNotFound
}

func (f *fakeReportRepository) Query(_ context.Context, q report.Query) (report.Page, error) {
	f.lastQuery = q
	if f.callIdx >= len(f.pages) {
		return report.Page{}, nil
	}
	p := f.pages[f.callIdx]
	f.callIdx++
	return p, nil
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errNotFound = fakeErr("nicht gefunden")

func testReport(t *testing.T, id report.ReportID, reportID string) report.AggregateReport {
	t.Helper()
	domain, err := report.NewDomainName("example.com")
	require.NoError(t, err)
	dr, err := report.NewDateRange(time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	require.NoError(t, err)
	r, err := report.NewAggregateReport(
		report.Metadata{OrgName: "org.example", ReportID: reportID, Range: dr},
		policy, nil, report.SourceReference{}, time.Now(),
	)
	require.NoError(t, err)
	r.ID = id
	return *r
}

func newSyncTestView(repo *fakeReportRepository, window fyne.Window) *View {
	v := NewView(&queryreports.UseCase{Reports: repo}, window)
	v.runBackground = func(f func()) { f() }
	return v
}

func TestView_Reload_NoReports_ShowsEmptyState(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{{}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	require.NotNil(t, uitest.FindLabel(v, i18n.ReportsEmptyTitle))
}

func TestView_Reload_WithReports_HidesEmptyState(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{
		{Reports: []report.AggregateReport{testReport(t, 1, "r1")}},
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	require.Nil(t, uitest.FindLabel(v, i18n.ReportsEmptyTitle))
	require.Len(t, v.data, 1)
}

func TestView_LoadMore_AppendsSecondPage(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{
		{Reports: []report.AggregateReport{testReport(t, 1, "r1")}, NextCursor: "cursor-1"},
		{Reports: []report.AggregateReport{testReport(t, 2, "r2")}},
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()
	require.Len(t, v.data, 1)
	require.True(t, v.loadMore.Visible(), "solange NextCursor gesetzt ist, muss 'Weitere laden' sichtbar sein")

	v.loadMoreReports()
	require.Len(t, v.data, 2)
	require.False(t, v.loadMore.Visible(), "ohne weiteren Cursor darf der Button nicht mehr sichtbar sein")
}

func TestView_SetFilter_ForwardsPeriodAndDomainToQuery(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{{}}}
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

func TestView_SetFilter_NoMatches_ShowsFilterSpecificEmptyText(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{{}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)

	v.SetFilter(nil, "andere-domain.example")

	require.NotNil(t, uitest.FindLabel(v, i18n.ReportsEmptyNoMatch))
}

func TestView_Reload_ResetsPreviousData(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{
		{Reports: []report.AggregateReport{testReport(t, 1, "r1")}, NextCursor: "cursor-1"},
	}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()
	require.Len(t, v.data, 1)

	repo.callIdx = 0
	repo.pages = []report.Page{{Reports: []report.AggregateReport{testReport(t, 2, "r2")}}}
	v.Reload()

	require.Len(t, v.data, 1)
	require.Equal(t, "r2", v.data[0].Metadata.ReportID)
}

func TestNewView_ConstructsWithoutPanicking_GroupSelectDefaultDoesNotTriggerPrematureLoad(t *testing.T) {
	// Regressionstest: widget.Select.SetSelected() löst OnChanged synchron
	// aus. Wird OnChanged vor v.container gesetzt, greift der erste
	// Aufruf über Reload()/setCenter() auf ein noch nicht zugewiesenes
	// v.container zu (Nil-Pointer-Panic direkt im Konstruktor).
	repo := &fakeReportRepository{}
	w := test.NewWindow(nil)
	defer w.Close()

	require.NotPanics(t, func() {
		v := NewView(&queryreports.UseCase{Reports: repo}, w)
		w.SetContent(v)
	})
}

func TestView_GroupSelected_ForwardsGroupByToQueryAndReloads(t *testing.T) {
	repo := &fakeReportRepository{pages: []report.Page{{}, {}}}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	v.group.SetSelected(i18n.ReportsGroupDomain)

	require.Equal(t, report.GroupByDomain, repo.lastQuery.GroupBy)
}

func TestView_ShowDetail_LoadsFullReport(t *testing.T) {
	summary := testReport(t, 1, "r1")
	full := summary
	full.Records = []report.Record{}

	repo := &fakeReportRepository{
		pages: []report.Page{{Reports: []report.AggregateReport{summary}}},
		byID:  map[report.ReportID]*report.AggregateReport{1: &full},
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()

	require.NotPanics(t, func() { v.showDetail(summary) })
}

// TestView_ShowDetail_DialogCanBeClosed ist ein Regressionstest: der
// Bericht-Detaildialog lief auf dialog.NewCustomWithoutButtons — ohne
// jeden Knopf und ohne dass irgendein Code-Pfad Hide() aufrief. Anders
// als ein gewöhnliches Popup schließt ein widget.NewModalPopUp (das
// dialog.NewCustom* intern verwendet) NICHT durch Antippen außerhalb —
// der Dialog blieb für den Nutzer dauerhaft offen, ohne jede Möglichkeit
// ihn zu schließen.
func TestView_ShowDetail_DialogCanBeClosed(t *testing.T) {
	summary := testReport(t, 1, "r1")
	full := summary
	full.Records = []report.Record{}

	repo := &fakeReportRepository{
		pages: []report.Page{{Reports: []report.AggregateReport{summary}}},
		byID:  map[report.ReportID]*report.AggregateReport{1: &full},
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(repo, w)
	w.SetContent(v)
	v.Reload()
	v.showDetail(summary)

	overlay := w.Canvas().Overlays().Top()
	require.NotNil(t, overlay, "Detaildialog muss als Overlay sichtbar sein")

	closeButton := uitest.FindButton(t, overlay, i18n.ButtonClose)
	test.Tap(closeButton)

	require.Nil(t, w.Canvas().Overlays().Top(), "Dialog muss nach Tippen auf %q schließen", i18n.ButtonClose)
}
