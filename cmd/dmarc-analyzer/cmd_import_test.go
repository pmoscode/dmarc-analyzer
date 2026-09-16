package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var (
	fixedBegin   = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	hourDuration = time.Hour
)

func testAppForImport() (*app, *fakeReportRepository) {
	reports := newFakeReportRepository()
	return &app{
		importer: &importfiles.UseCase{
			Reports: reports,
			Decoder: fakeDecoder{},
			Parsers: []domainsync.ReportParser{fakeParser{}},
		},
	}, reports
}

func TestRunImport_NoPaths_ReturnsError(t *testing.T) {
	t.Parallel()

	a, _ := testAppForImport()
	err := runImport(context.Background(), a, nil)
	require.Error(t, err)
}

func TestRunImport_ImportsGivenFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "report.xml")
	require.NoError(t, os.WriteFile(path, []byte("report-1"), 0o600))

	a, reports := testAppForImport()
	err := runImport(context.Background(), a, []string{path})
	require.NoError(t, err)
	require.Len(t, reports.byKey, 1)
}
