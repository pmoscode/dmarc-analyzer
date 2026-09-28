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
	errTest    = errors.New("test error")
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

	// Non-.eml files go to the parser unchanged as one attachment — the
	// file content itself controls here (via fakeParser) which report
	// results.
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
	_, err := uc.ImportFile(context.Background(), "/path/does/not/exist.xml")
	require.Error(t, err)
}

func TestImportFile_ParseError_RecordsFailure(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "broken.xml", "broken-report")

	uc, deps := newTestUseCase(map[string]error{"broken-report": errTest})
	result, err := uc.ImportFile(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count())
}

func TestImportFile_DuplicateReport_CountsAsSkipped(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "report.xml", "same-report")
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
	require.Equal(t, 1, result.New, "the good file must still be imported despite the missing one")
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
	require.Zero(t, deps.reports.count(), "nothing may be imported when the context is already canceled")
}

func TestImportFile_ZeroRecordsReport_IsImportedFineToo(t *testing.T) {
	// Edge case from IMPLEMENTIERUNG.md section 12.3: a report without
	// records is a valid business case (fakeParser never returns records
	// here anyway, but this is representative of the path).
	t.Parallel()

	path := writeTestFile(t, "empty.xml", "report-without-records")
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

	uc, deps := newTestUseCase(map[string]error{"broken-report": errTest})
	result, err := uc.ImportData(context.Background(), "broken.xml", []byte("broken-report"))
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count())
}

func TestImportData_DuplicateReport_CountsAsSkipped(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(nil)
	data := []byte("same-report")

	_, err := uc.ImportData(context.Background(), "report.xml", data)
	require.NoError(t, err)

	result, err := uc.ImportData(context.Background(), "report.xml", data)
	require.NoError(t, err)
	require.Zero(t, result.New)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, deps.reports.count())
}

func TestImportData_SameContentAsImportFile_ProducesEquivalentResult(t *testing.T) {
	// After the switch to ImportData (see usecase.go), ImportFile must
	// return exactly the same result as ImportData with already-read
	// bytes — no behavior change from the extraction, just a new entry
	// point (MIGRATIONSPLAN.md extension 9.3).
	t.Parallel()

	path := writeTestFile(t, "report.xml", "identical-content")
	ucFile, depsFile := newTestUseCase(nil)
	fileResult, err := ucFile.ImportFile(context.Background(), path)
	require.NoError(t, err)

	ucData, depsData := newTestUseCase(nil)
	dataResult, err := ucData.ImportData(context.Background(), "report.xml", []byte("identical-content"))
	require.NoError(t, err)

	require.Equal(t, fileResult, dataResult)
	require.Equal(t, depsFile.reports.count(), depsData.reports.count())
}

func TestImportData_MultiReportParser_ImportsAllReportsFromOneAttachment(t *testing.T) {
	// Regression: importAttachments used to call only Parse(), which per
	// the domainsync.ReportParser contract returns only ONE report — an
	// attachment (e.g. a .zip) that can return multiple reports via
	// domainsync.MultiReportParser was thereby imported only a fifth of
	// the way (the first report). See also internal/web
	// TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth for the
	// same bug with the real dmarcxml.Parser.
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
