package importfiles_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var (
	fixedBegin = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	hour       = time.Hour
	errTest    = errors.New("testfehler")
)

type testDeps struct {
	reports       *fakeReportRepository
	failedImports *fakeFailedImportRepository
}

func newTestUseCase(parserFailFor map[string]error) (*importfiles.UseCase, *testDeps) {
	deps := &testDeps{
		reports:       newFakeReportRepository(),
		failedImports: newFakeFailedImportRepository(),
	}
	uc := &importfiles.UseCase{
		Reports:       deps.reports,
		FailedImports: deps.failedImports,
		Decoder:       fakeDecoder{},
		Parsers:       []domainsync.ReportParser{fakeParser{failFor: parserFailFor}},
	}
	return uc, deps
}

func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestImportFile_XMLFile_ImportsAsSingleAttachment(t *testing.T) {
	t.Parallel()

	// Nicht-.eml-Dateien gehen unverändert als ein Anhang an den Parser —
	// der Dateiinhalt selbst steuert hier (über fakeParser), welcher
	// Report entsteht.
	path := writeTestFile(t, "report.xml", "report-from-xml")

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, 1, result.New)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportFile_EmlFile_DecodesFirst(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "mail.eml", "report-from-eml")

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, 1, result.New)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportFile_UnsupportedAttachment_SkippedSilently(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "irrelevant.txt", "unsupported")

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Zero(t, result.New)
	require.Zero(t, result.Failed)
	require.Zero(t, deps.reports.count())
}

func TestImportFile_NonexistentFile_ReturnsError(t *testing.T) {
	t.Parallel()

	uc, _ := newTestUseCase(nil)
	_, err := uc.ImportFile(context.Background(), "/pfad/existiert/nicht.xml")
	require.Error(t, err)
}

func TestImportFile_ParseError_RecordsFailure(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "kaputt.xml", "kaputter-report")

	uc, deps := newTestUseCase(map[string]error{"kaputter-report": errTest})
	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count())
}

func TestImportFile_DuplicateReport_CountsAsSkipped(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "report.xml", "gleicher-report")
	uc, deps := newTestUseCase(nil)

	_, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)

	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Zero(t, result.New)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportPaths_AggregatesAcrossMultipleFiles(t *testing.T) {
	t.Parallel()

	a := writeTestFile(t, "a.xml", "report-a")
	b := writeTestFile(t, "b.xml", "report-b")

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportPaths(context.Background(), []string{a, b})
	require.NoError(t, err)
	require.Equal(t, 2, result.New)
	require.Equal(t, 2, deps.reports.count())
}

func TestImportPaths_OneFileFails_OthersStillProcessed(t *testing.T) {
	t.Parallel()

	good := writeTestFile(t, "good.xml", "report-good")
	missing := filepath.Join(t.TempDir(), "does-not-exist.xml")

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportPaths(context.Background(), []string{missing, good})
	require.NoError(t, err)
	require.Equal(t, 1, result.New, "die gute Datei muss trotz der fehlenden importiert werden")
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportPaths_ContextCancelled_StopsEarly(t *testing.T) {
	t.Parallel()

	a := writeTestFile(t, "a.xml", "report-a")
	b := writeTestFile(t, "b.xml", "report-b")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	uc, deps := newTestUseCase(nil)
	_, err := uc.ImportPaths(ctx, []string{a, b})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, deps.reports.count(), "bei sofort abgebrochenem Kontext darf nichts importiert werden")
}

func TestImportFile_ZeroRecordsReport_IsImportedFineToo(t *testing.T) {
	// Randfall aus IMPLEMENTIERUNG.md Abschnitt 12.3: ein Report ohne
	// Records ist fachlich gültig (fakeParser liefert hier ohnehin nie
	// Records, aber das ist repräsentativ für den Pfad).
	t.Parallel()

	path := writeTestFile(t, "leer.xml", "report-ohne-records")
	uc, _ := newTestUseCase(nil)

	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, 1, result.New)
}
