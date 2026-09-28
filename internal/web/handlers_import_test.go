package web

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/mailmime"
)

// newTestServerWithImporter builds a server with a real
// importfiles.UseCase (real dmarcxml.Parser/mailmime.Decoder — both are
// pure, I/O-free parsers; a fake would hide exactly the interesting
// behavior here: whether a sample file uploaded via POST /import is
// actually recognized and saved as a DMARC report).
func newTestServerWithImporter(t *testing.T) (*Server, *fakeReportRepository, *fakeFailedImportRepository) {
	t.Helper()

	reports := &fakeReportRepository{}
	failed := &fakeFailedImportRepository{}

	deps := Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{}},
		Importer: &importfiles.UseCase{
			Reports:       reports,
			FailedImports: failed,
			Decoder:       mailmime.NewDecoder(),
			Parsers:       []domainsync.ReportParser{dmarcxml.NewParser()},
		},
	}
	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv, reports, failed
}

// multipartUpload builds a multipart/form-data request with one or more
// files under the field name "dateien" (see handlers_import.go).
func multipartUpload(t *testing.T, csrfToken string, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	require.NoError(t, mw.WriteField("csrf_token", csrfToken))
	for name, data := range files {
		part, err := mw.CreateFormFile("dateien", name)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	return body, mw.FormDataContentType()
}

func postMultipart(t *testing.T, client *http.Client, addr string, body *bytes.Buffer, contentType string) *http.Response {
	t.Helper()
	req := newRequest(t, http.MethodPost, "http://"+addr+"/import", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", "http://"+testPublicHost)

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func sampleReportXML(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/reports/rfc7489/sample.xml")
	require.NoError(t, err)
	return data
}

func TestHandleImportForm_RendersDropzone(t *testing.T) {
	srv, _, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/import")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `id="import-dropzone"`)
}

func TestHandleImportSubmit_ValidXML_ImportsAndShowsResult(t *testing.T) {
	srv, reports, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	csrf := csrfTokenFor(t, srv, client)
	body, contentType := multipartUpload(t, csrf, map[string][]byte{"report.xml": sampleReportXML(t)})

	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "1 new, 0 skipped, 0 failed.")
	require.Equal(t, 1, reports.count())
}

func TestHandleImportSubmit_MultipleFiles_AggregatesResult(t *testing.T) {
	srv, reports, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	sample := sampleReportXML(t)
	body, contentType := multipartUpload(t, csrfTokenFor(t, srv, client), map[string][]byte{
		"a.xml": sample,
		"b.xml": append(append([]byte{}, sample...), []byte(" ")...), // minimally different content
	})

	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "2 new, 0 skipped, 0 failed.")
	require.Equal(t, 2, reports.count())
}

func TestHandleImportSubmit_CorruptFile_CountsAsFailed(t *testing.T) {
	srv, _, failed := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	body, contentType := multipartUpload(t, csrfTokenFor(t, srv, client), map[string][]byte{
		"broken.xml": []byte("this is not valid dmarc xml"),
	})

	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "0 new, 0 skipped, 1 failed.")
	require.Equal(t, 1, failed.count())
}

func TestHandleImportSubmit_NoFiles_ShowsError(t *testing.T) {
	srv, _, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	body, contentType := multipartUpload(t, csrfTokenFor(t, srv, client), nil)
	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "Please select at least one file.")
}

func TestHandleImportSubmit_FileTooLarge_CountsAsFailed(t *testing.T) {
	srv, _, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	huge := bytes.Repeat([]byte("x"), maxImportFileSize+1)
	body, contentType := multipartUpload(t, csrfTokenFor(t, srv, client), map[string][]byte{"huge.xml": huge})

	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "0 new, 0 skipped, 1 failed.")
}

func TestHandleImportSubmit_MissingCSRFToken_Returns403(t *testing.T) {
	srv, _, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	body, contentType := multipartUpload(t, "", map[string][]byte{"report.xml": sampleReportXML(t)})
	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth(t *testing.T) {
	// MIGRATIONSPLAN.md M4 "done when": "a .zip with several reports can
	// be imported via drag & drop" — drag & drop itself is a native
	// browser/HTML mechanism (not testable server-side), but this is the
	// decisive part: an uploaded .zip containing several reports gets
	// fully imported via the real dmarcxml parser.
	srv, reports, _ := newTestServerWithImporter(t)
	client := authenticatedClient(t, srv)

	zipData, err := os.ReadFile("../../testdata/reports/multi/two_reports.zip")
	require.NoError(t, err)

	body, contentType := multipartUpload(t, csrfTokenFor(t, srv, client), map[string][]byte{"two_reports.zip": zipData})
	resp := postMultipart(t, client, srv.Addr(), body, contentType)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(html), "2 new, 0 skipped, 0 failed.")
	require.Equal(t, 2, reports.count())
}
