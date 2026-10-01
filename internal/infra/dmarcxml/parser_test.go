package dmarcxml_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
)

// newTestParser returns a Parser with a fixed clock, so ImportedAt is
// deterministic in tests.
func newTestParser() *dmarcxml.Parser {
	fixed := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	return &dmarcxml.Parser{Clock: func() time.Time { return fixed }}
}

func loadFixture(t *testing.T, relPath string) sync.RawAttachment {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "reports", relPath))
	require.NoError(t, err, "failed to read fixture %q", relPath)

	return sync.RawAttachment{
		Filename: filepath.Base(relPath),
		Data:     data,
	}
}

// Golden-file tests: one fixture per provider quirk, each must successfully
// convert to an AggregateReport (IMPLEMENTIERUNG.md section 12.2).

func TestParse_RFC7489CanonicalExample(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := loadFixture(t, "rfc7489/sample.xml")
	require.True(t, p.Supports(attachment))

	r, err := p.Parse(context.Background(), attachment)
	require.NoError(t, err)

	require.Equal(t, "mail.example.org", r.Metadata.OrgName)
	require.Equal(t, "9391651994964116463", r.Metadata.ReportID)
	require.Equal(t, "example.com", r.Policy.Domain.String())
	require.Equal(t, report.PolicyReject, r.Policy.Policy)
	require.Equal(t, 100, r.Policy.Percentage)
	require.Len(t, r.Records, 2)

	require.Equal(t, "203.0.113.5", r.Records[0].SourceIP.String())
	require.Equal(t, 2, r.Records[0].Count)
	require.Equal(t, report.DispositionNone, r.Records[0].Evaluated.Disposition)
	require.Len(t, r.Records[0].Auth.SPF, 1)
	require.Equal(t, "mfrom", r.Records[0].Auth.SPF[0].Scope, "scope is missing from the XML, the RFC default of mfrom must apply")

	require.Equal(t, report.DispositionReject, r.Records[1].Evaluated.Disposition)
	require.Len(t, r.Records[1].Evaluated.Reasons, 1)
	require.Equal(t, "forwarded", r.Records[1].Evaluated.Reasons[0].Type)
	require.Equal(t, "mfrom", r.Records[1].Auth.SPF[0].Scope, "explicit scope must be preserved, not overwritten by the default")
}

func TestParse_GoogleStyle_MissingAlignmentDefaultsToRelaxed(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := loadFixture(t, "google/sample.xml.gz")
	require.True(t, p.Supports(attachment))

	r, err := p.Parse(context.Background(), attachment)
	require.NoError(t, err)

	require.Equal(t, report.AlignmentRelaxed, r.Policy.DKIMAlignment)
	require.Equal(t, report.AlignmentRelaxed, r.Policy.SPFAlignment)
	require.Len(t, r.Records, 1)
}

func TestParse_MicrosoftStyle_ZipUppercaseEnumsAndMissingPct(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := loadFixture(t, "microsoft/sample.zip")
	require.True(t, p.Supports(attachment))

	r, err := p.Parse(context.Background(), attachment)
	require.NoError(t, err)

	require.Equal(t, 100, r.Policy.Percentage, "pct is missing from the XML, the RFC default of 100 must apply")
	require.Equal(t, report.PolicyNone, r.Policy.Policy, "uppercase 'NONE' must be recognized")
	require.Equal(t, report.AlignmentStrict, r.Policy.DKIMAlignment)

	require.Len(t, r.Records, 1)
	require.Equal(t, report.DispositionNone, r.Records[0].Evaluated.Disposition)
	require.Len(t, r.Records[0].Auth.DKIM, 2, "both DKIM signatures must be preserved")
	require.Equal(t, report.AuthResultFail, r.Records[0].Auth.DKIM[1].Result)
}

func TestParseAll_ZipWithMultipleReports(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := loadFixture(t, "multi/two_reports.zip")

	reports, err := p.ParseAll(context.Background(), attachment)
	require.NoError(t, err)
	require.Len(t, reports, 2)

	ids := []string{reports[0].Metadata.ReportID, reports[1].Metadata.ReportID}
	require.ElementsMatch(t, []string{"multi-report-a", "multi-report-b"}, ids)
}

func TestParse_MissingPct_DefaultsTo100(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	r, err := p.Parse(context.Background(), loadFixture(t, "quirky/missing_pct.xml"))
	require.NoError(t, err)
	require.Equal(t, 100, r.Policy.Percentage)
}

func TestParse_UnknownEnumValues_AreMappedNotDiscarded(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	r, err := p.Parse(context.Background(), loadFixture(t, "quirky/unknown_enums.xml"))
	require.NoError(t, err, "unknown enum values must not cause the import to fail")

	require.Equal(t, report.PolicyUnknown, r.Policy.Policy)
	require.Equal(t, report.PolicyUnknown, r.Policy.SubdomainPolicy)
	require.Len(t, r.Records, 1)
	require.Equal(t, report.DispositionUnknown, r.Records[0].Evaluated.Disposition)
	require.Equal(t, report.AuthResultUnknown, r.Records[0].Evaluated.DKIM)
	require.Equal(t, report.AuthResultUnknown, r.Records[0].Auth.DKIM[0].Result)
}

func TestParse_ZeroRecords(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	r, err := p.Parse(context.Background(), loadFixture(t, "quirky/zero_records.xml"))
	require.NoError(t, err)
	require.Empty(t, r.Records)
}

func TestParse_ContextCancelled_ReturnsImmediately(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := newTestParser()
	_, err := p.Parse(ctx, loadFixture(t, "rfc7489/sample.xml"))
	require.ErrorIs(t, err, context.Canceled)
}

func TestSupports_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	require.False(t, p.Supports(sync.RawAttachment{Filename: "readme.txt", Data: []byte("hello")}))
}

func TestSupports_MagicBytesFallbackForUnnamedAttachment(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	data := []byte(`<?xml version="1.0"?><feedback></feedback>`)
	require.True(t, p.Supports(sync.RawAttachment{Filename: "9391651994964116463", Data: data}))
}

func TestParse_MalformedXML_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := sync.RawAttachment{
		Filename: "broken.xml",
		Data:     []byte("<feedback><report_metadata><org_name>"),
	}

	_, err := p.Parse(context.Background(), attachment)
	require.Error(t, err)
}

func TestParse_CorruptGzip_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newTestParser()
	attachment := sync.RawAttachment{
		Filename: "broken.xml.gz",
		Data:     []byte{0x1f, 0x8b, 0x00, 0x00, 0x00}, // Gzip magic bytes, but no valid gzip data after.
	}

	_, err := p.Parse(context.Background(), attachment)
	require.Error(t, err)
}

func TestParse_ZipWithoutXML_ReturnsError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("readme.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("no xml here"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	p := newTestParser()
	attachment := sync.RawAttachment{Filename: "empty.zip", Data: buf.Bytes()}

	_, err = p.Parse(context.Background(), attachment)
	require.Error(t, err)
}

// TestParse_ZipBomb_RejectedByDefaultSizeLimit creates a zip whose unpacked
// content is far above the 100 MB limit, from highly compressed zero bytes
// — the zip file itself stays small
// (IMPLEMENTIERUNG.md section 16: "zip bomb in attachment").
func TestParse_ZipBomb_RejectedByDefaultSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("generates >100 MB of test data, see task test:unit")
	}
	t.Parallel()

	const oversized = 101 * 1024 * 1024 // 101 MB, > 100 MB limit

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "bomb.xml", Method: zip.Deflate})
	require.NoError(t, err)
	_, err = w.Write(make([]byte, oversized))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	p := newTestParser()
	attachment := sync.RawAttachment{Filename: "bomb.zip", Data: buf.Bytes()}

	_, err = p.Parse(context.Background(), attachment)
	require.ErrorIs(t, err, dmarcxml.ErrAttachmentTooLarge)
}

func TestParse_GzipBomb_RejectedByDefaultSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("generates >100 MB of test data, see task test:unit")
	}
	t.Parallel()

	const oversized = 101 * 1024 * 1024

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(make([]byte, oversized))
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	p := newTestParser()
	attachment := sync.RawAttachment{Filename: "bomb.xml.gz", Data: buf.Bytes()}

	_, err = p.Parse(context.Background(), attachment)
	require.ErrorIs(t, err, dmarcxml.ErrAttachmentTooLarge)
}

// TestParse_XXEIsNotResolved documents that external entities are not
// resolved. encoding/xml fundamentally does not support DTD resolution — a
// document with a non-predefined entity is rejected outright, rather than
// silently ignoring or even resolving the entity. This test pins down both
// properties in case the standard library's behavior ever changes
// (IMPLEMENTIERUNG.md section 12.3).
func TestParse_XXEIsNotResolved(t *testing.T) {
	t.Parallel()

	secretFile := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret"), 0o600))

	xxe := `<?xml version="1.0"?>
<!DOCTYPE feedback [
  <!ENTITY xxe SYSTEM "file://` + secretFile + `">
]>
<feedback>
  <report_metadata>
    <org_name>&xxe;</org_name>
    <email>dmarc@example.com</email>
    <report_id>xxe-test</report_id>
    <date_range><begin>1735689600</begin><end>1735776000</end></date_range>
  </report_metadata>
  <policy_published><domain>example.com</domain><p>reject</p><pct>100</pct></policy_published>
</feedback>`

	p := newTestParser()
	r, err := p.Parse(context.Background(), sync.RawAttachment{
		Filename: "xxe.xml",
		Data:     []byte(xxe),
	})

	require.Error(t, err, "a document with an external entity must be rejected, not silently processed")
	require.Nil(t, r)
	require.NotContains(t, err.Error(), "secret",
		"the secret file content must not even appear in the error message")
}
