package web

import (
	"context"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
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
	return nil, nil
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
