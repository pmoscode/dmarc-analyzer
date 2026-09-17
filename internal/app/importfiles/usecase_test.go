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

func TestImportData_XMLBytes_ImportsAsSingleAttachment(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportData(context.Background(), "report.xml", []byte("report-from-bytes"))
	require.NoError(t, err)
	require.Equal(t, 1, result.New)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportData_EmlBytes_Decodes(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(nil)
	result, err := uc.ImportData(context.Background(), "mail.eml", []byte("report-from-eml-bytes"))
	require.NoError(t, err)
	require.Equal(t, 1, result.New)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportData_ParseError_RecordsFailure(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(map[string]error{"kaputter-report": errTest})
	result, err := uc.ImportData(context.Background(), "kaputt.xml", []byte("kaputter-report"))
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count())
}

func TestImportData_DuplicateReport_CountsAsSkipped(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(nil)
	data := []byte("gleicher-report")

	_, err := uc.ImportData(context.Background(), "report.xml", data)
	require.NoError(t, err)

	result, err := uc.ImportData(context.Background(), "report.xml", data)
	require.NoError(t, err)
	require.Zero(t, result.New)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportData_SameContentAsImportFile_ProducesEquivalentResult(t *testing.T) {
	// ImportFile muss nach der Umstellung auf ImportData (siehe
	// usecase.go) exakt dasselbe Ergebnis liefern wie ImportData mit
	// bereits gelesenen Bytes — keine Verhaltensänderung durch die
	// Extraktion, nur ein neuer Einstiegspunkt (MIGRATIONSPLAN.md
	// Erweiterung 9.3).
	t.Parallel()

	path := writeTestFile(t, "report.xml", "identischer-inhalt")
	ucFile, depsFile := newTestUseCase(nil)
	fileResult, err := ucFile.ImportFile(context.Background(), path)
	require.NoError(t, err)

	ucData, depsData := newTestUseCase(nil)
	dataResult, err := ucData.ImportData(context.Background(), "report.xml", []byte("identischer-inhalt"))
	require.NoError(t, err)

	require.Equal(t, fileResult, dataResult)
	require.Equal(t, depsFile.reports.count(), depsData.reports.count())
}

func TestImportData_MultiReportParser_ImportsAllReportsFromOneAttachment(t *testing.T) {
	// Regression: importAttachments rief zuvor ausschließlich Parse()
	// auf, das laut domainsync.ReportParser-Vertrag nur EINEN Report
	// liefert — ein Anhang (z. B. ein .zip), der über
	// domainsync.MultiReportParser mehrere Reports zurückgeben kann,
	// wurde dadurch nur zu einem Fünftel (dem ersten Report) importiert.
	// Siehe auch internal/web TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth
	// für denselben Fehler mit dem echten dmarcxml.Parser.
	t.Parallel()

	uc, deps := newTestUseCase(nil)
	uc.Parsers = []domainsync.ReportParser{fakeMultiParser{}}

	result, err := uc.ImportData(context.Background(), "multi.zip", []byte("multi:report-a,report-b,report-c"))
	require.NoError(t, err)
	require.Equal(t, 3, result.New)
	require.Zero(t, result.Skipped)
	require.Zero(t, result.Failed)
	require.Equal(t, 3, deps.reports.count())
}
