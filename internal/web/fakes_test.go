package web

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
)

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errTest = fakeErr("testfehler")

// fakeRepository implementiert analysis.Repository mit fest verdrahteten
// Rückgabewerten — derselbe Aufbau wie zuvor internal/ui/dashboard
// (fakes_test.go), hier eigenständig für internal/web.
type fakeRepository struct {
	stats        analysis.Statistics
	dailyVolumes []analysis.DailyVolume
	topSources   []analysis.SourceVolume
	heatmap      analysis.Heatmap
	computeErr   error

	// lastDailyVolumesQuery hält die zuletzt an DailyVolumes übergebene
	// Query fest — für Tests, die prüfen, dass die URL-Filterleiste
	// (zeitraum/domain, MIGRATIONSPLAN.md Meilenstein M1) tatsächlich bis
	// zum Repository durchgereicht wird, statt nur am Handler zu enden.
	lastDailyVolumesQuery analysis.Query
}

func (f *fakeRepository) Compute(context.Context, analysis.Query) (analysis.Statistics, error) {
	if f.computeErr != nil {
		return analysis.Statistics{}, f.computeErr
	}
	return f.stats, nil
}

func (f *fakeRepository) DailyVolumes(_ context.Context, q analysis.Query) ([]analysis.DailyVolume, error) {
	f.lastDailyVolumesQuery = q
	return f.dailyVolumes, nil
}

func (f *fakeRepository) TopSources(context.Context, analysis.Query, int) ([]analysis.SourceVolume, error) {
	return f.topSources, nil
}

func (f *fakeRepository) Heatmap(context.Context, analysis.Query, int) (analysis.Heatmap, error) {
	return f.heatmap, nil
}

func mustSourceIP(s string) report.SourceIP {
	ip, err := report.NewSourceIP(s)
	if err != nil {
		panic(err)
	}
	return ip
}

// fakeReportRepository implementiert report.Repository mit fest
// verdrahteten Rückgabewerten — für Tests von /berichte, /berichte/seite
// und /berichte/{id}. Save/Exists werden von queryreports.UseCase nicht
// benutzt, müssen aber für das Interface vorhanden sein.
type fakeReportRepository struct {
	page      report.Page
	queryErr  error
	byID      map[report.ReportID]*report.AggregateReport
	getErr    error
	lastQuery report.Query
}

func (f *fakeReportRepository) Save(context.Context, *report.AggregateReport) error {
	return nil
}

func (f *fakeReportRepository) Exists(context.Context, report.Key) (bool, error) {
	return false, nil
}

func (f *fakeReportRepository) FindByID(_ context.Context, id report.ReportID) (*report.AggregateReport, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	full, ok := f.byID[id]
	if !ok {
		return nil, fakeErr("nicht gefunden")
	}
	return full, nil
}

func (f *fakeReportRepository) Query(_ context.Context, q report.Query) (report.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return report.Page{}, f.queryErr
	}
	return f.page, nil
}

// fakeSourcesRepository implementiert domainsources.Repository mit fest
// verdrahteten Rückgabewerten — für Tests von /quellen und /quellen/seite.
type fakeSourcesRepository struct {
	page     domainsources.Page
	queryErr error

	lastQuery domainsources.Query
}

func (f *fakeSourcesRepository) Query(_ context.Context, q domainsources.Query) (domainsources.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return domainsources.Page{}, f.queryErr
	}
	return f.page, nil
}

// fakeEnricher liefert für jede Quell-IP dieselbe fest verdrahtete
// Anreicherung (leer, solange nichts anderes konfiguriert ist) — echte
// PTR-/Dienst-Erkennung ist Netzwerk-I/O und hat in Handler-Tests nichts
// zu suchen.
type fakeEnricher struct {
	enrichment domainsources.Enrichment
}

func (f *fakeEnricher) Enrich(context.Context, report.SourceIP) domainsources.Enrichment {
	return f.enrichment
}
