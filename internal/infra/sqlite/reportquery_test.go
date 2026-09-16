package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func saveTestReport(t testing.TB, repo *sqlite.ReportRepository, opts reportOpts) {
	t.Helper()
	r := newTestReport(t, opts)
	require.NoError(t, repo.Save(context.Background(), r))
}

func TestQuery_FiltersByDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{domain: "a.example", reportID: "a-1"})
	saveTestReport(t, repo, reportOpts{domain: "b.example", reportID: "b-1"})

	page, err := repo.Query(ctx, report.Query{Domain: "a.example"})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Equal(t, "a.example", page.Reports[0].Policy.Domain.String())
}

func TestQuery_FiltersByOrgName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{orgName: "org-a.example", reportID: "oa-1"})
	saveTestReport(t, repo, reportOpts{orgName: "org-b.example", reportID: "ob-1"})

	page, err := repo.Query(ctx, report.Query{OrgName: "org-b.example"})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Equal(t, "org-b.example", page.Reports[0].Metadata.OrgName)
}

func TestQuery_FiltersByPeriod_Overlap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	saveTestReport(t, repo, reportOpts{reportID: "period-jan", begin: jan, end: feb})
	saveTestReport(t, repo, reportOpts{reportID: "period-feb", begin: feb, end: mar})

	// End bewusst hinter feb (nicht exakt feb): date_begin < End soll den
	// bei feb beginnenden zweiten Report mit einschließen. .Unix() rundet
	// auf volle Sekunden, ein Sub-Sekunden-Versatz würde dort verloren
	// gehen.
	period := report.DateRange{Begin: jan, End: feb.Add(time.Second)}
	page, err := repo.Query(ctx, report.Query{Period: &period})
	require.NoError(t, err)
	require.Len(t, page.Reports, 2, "beide Reports überlappen mit dem Zeitraum jan bis feb+1s")

	strictlyJan := report.DateRange{Begin: jan, End: jan.Add(time.Hour)}
	page, err = repo.Query(ctx, report.Query{Period: &strictlyJan})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Equal(t, "period-jan", page.Reports[0].Metadata.ReportID)
}

func TestQuery_FiltersBySourceIP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{reportID: "ip-a", sourceIP: "203.0.113.9"})
	saveTestReport(t, repo, reportOpts{reportID: "ip-b", sourceIP: "198.51.100.9"})

	page, err := repo.Query(ctx, report.Query{SourceIP: "198.51.100.9"})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Equal(t, "ip-b", page.Reports[0].Metadata.ReportID)
}

func TestQuery_FiltersByDisposition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{reportID: "disp-none", disposition: report.DispositionNone})
	saveTestReport(t, repo, reportOpts{reportID: "disp-reject", disposition: report.DispositionReject})

	page, err := repo.Query(ctx, report.Query{Disposition: report.DispositionReject})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Equal(t, "disp-reject", page.Reports[0].Metadata.ReportID)
}

func TestQuery_ResultsHaveNoRecordsLoaded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{reportID: "lazy-1"})

	page, err := repo.Query(ctx, report.Query{})
	require.NoError(t, err)
	require.Len(t, page.Reports, 1)
	require.Empty(t, page.Reports[0].Records,
		"Query lädt laut Portvertrag keine Records — siehe domain/report/repository.go")
}

func TestQuery_SortByDateBeginAscendingAndDescending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	saveTestReport(t, repo, reportOpts{reportID: "day-2", begin: day2, end: day2.Add(time.Hour)})
	saveTestReport(t, repo, reportOpts{reportID: "day-1", begin: day1, end: day1.Add(time.Hour)})
	saveTestReport(t, repo, reportOpts{reportID: "day-3", begin: day3, end: day3.Add(time.Hour)})

	asc, err := repo.Query(ctx, report.Query{SortField: report.SortByDateBegin, SortDirection: report.SortAscending})
	require.NoError(t, err)
	require.Equal(t, []string{"day-1", "day-2", "day-3"}, reportIDs(asc.Reports))

	desc, err := repo.Query(ctx, report.Query{SortField: report.SortByDateBegin, SortDirection: report.SortDescending})
	require.NoError(t, err)
	require.Equal(t, []string{"day-3", "day-2", "day-1"}, reportIDs(desc.Reports))
}

func TestQuery_GroupBySourceIP_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	_, err := repo.Query(ctx, report.Query{GroupBy: report.GroupBySourceIP})
	require.Error(t, err)
}

func TestQuery_GroupByOrg_KeepsGroupsAdjacent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	saveTestReport(t, repo, reportOpts{orgName: "org-b.example", reportID: "b-1"})
	saveTestReport(t, repo, reportOpts{orgName: "org-a.example", reportID: "a-1"})
	saveTestReport(t, repo, reportOpts{orgName: "org-b.example", reportID: "b-2"})
	saveTestReport(t, repo, reportOpts{orgName: "org-a.example", reportID: "a-2"})

	page, err := repo.Query(ctx, report.Query{GroupBy: report.GroupByOrg, SortField: report.SortByDateBegin})
	require.NoError(t, err)
	require.Len(t, page.Reports, 4)

	orgs := make([]string, len(page.Reports))
	for i, r := range page.Reports {
		orgs[i] = r.Metadata.OrgName
	}
	// Gleiche Organisation muss zusammenstehen, unabhängig vom Datum.
	require.Equal(t, []string{"org-a.example", "org-a.example", "org-b.example", "org-b.example"}, orgs)
}

func TestQuery_KeysetPagination_CoversAllReportsExactlyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := sqlite.NewReportRepository(newTestDB(t))

	const total = 25
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < total; i++ {
		begin := base.Add(time.Duration(i) * time.Hour)
		saveTestReport(t, repo, reportOpts{
			reportID: fmt.Sprintf("page-%02d", i),
			begin:    begin,
			end:      begin.Add(time.Minute),
		})
	}

	seen := make(map[string]bool)
	cursor := ""
	pages := 0
	for {
		page, err := repo.Query(ctx, report.Query{
			SortField: report.SortByDateBegin, SortDirection: report.SortAscending,
			Limit: 7, Cursor: cursor,
		})
		require.NoError(t, err)
		pages++

		for _, r := range page.Reports {
			require.False(t, seen[r.Metadata.ReportID], "Report %s doppelt gesehen", r.Metadata.ReportID)
			seen[r.Metadata.ReportID] = true
		}

		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		require.Less(t, pages, total, "Pagination terminiert nicht")
	}

	require.Len(t, seen, total)
	require.Equal(t, 4, pages, "25 Reports mit Limit 7 ergeben 4 Seiten (7+7+7+4)")
}

func reportIDs(reports []report.AggregateReport) []string {
	ids := make([]string, len(reports))
	for i, r := range reports {
		ids[i] = r.Metadata.ReportID
	}
	return ids
}
