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

// newTestParser liefert einen Parser mit fester Uhr, damit ImportedAt in
// Tests deterministisch ist.
func newTestParser() *dmarcxml.Parser {
	fixed := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	return &dmarcxml.Parser{Clock: func() time.Time { return fixed }}
}

func loadFixture(t *testing.T, relPath string) sync.RawAttachment {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "reports", relPath))
	require.NoError(t, err, "fixture %q konnte nicht gelesen werden", relPath)

	return sync.RawAttachment{
		Filename: filepath.Base(relPath),
		Data:     data,
	}
}

// Golden-File-Tests: je Provider-Eigenheit ein Fixture, jedes muss
// erfolgreich zu einem AggregateReport werden (IMPLEMENTIERUNG.md
// Abschnitt 12.2).

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

	require.Equal(t, report.DispositionReject, r.Records[1].Evaluated.Disposition)
	require.Len(t, r.Records[1].Evaluated.Reasons, 1)
	require.Equal(t, "forwarded", r.Records[1].Evaluated.Reasons[0].Type)
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

	require.Equal(t, 100, r.Policy.Percentage, "pct fehlt im XML, RFC-Default 100 muss greifen")
	require.Equal(t, report.PolicyNone, r.Policy.Policy, "Großschreibung 'NONE' muss erkannt werden")
	require.Equal(t, report.AlignmentStrict, r.Policy.DKIMAlignment)

	require.Len(t, r.Records, 1)
	require.Equal(t, report.DispositionNone, r.Records[0].Evaluated.Disposition)
	require.Len(t, r.Records[0].Auth.DKIM, 2, "zwei DKIM-Signaturen müssen beide erhalten bleiben")
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
	require.NoError(t, err, "unbekannte Enum-Werte dürfen den Import nicht scheitern lassen")

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
	require.False(t, p.Supports(sync.RawAttachment{Filename: "readme.txt", Data: []byte("hallo")}))
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
		Data:     []byte{0x1f, 0x8b, 0x00, 0x00, 0x00}, // Gzip-Magic-Bytes, aber kein gültiges Gzip danach.
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
	_, err = w.Write([]byte("kein xml hier"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	p := newTestParser()
	attachment := sync.RawAttachment{Filename: "empty.zip", Data: buf.Bytes()}

	_, err = p.Parse(context.Background(), attachment)
	require.Error(t, err)
}

// TestParse_ZipBomb_RejectedByDefaultSizeLimit erzeugt ein Zip, dessen
// entpackter Inhalt weit über dem 100-MB-Limit liegt, aus hochkomprimierten
// Nullbytes — die Zip-Datei selbst bleibt dabei klein
// (IMPLEMENTIERUNG.md Abschnitt 16: "Zip-Bombe im Anhang").
func TestParse_ZipBomb_RejectedByDefaultSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("erzeugt >100 MB Testdaten, siehe task test:unit")
	}
	t.Parallel()

	const oversized = 101 * 1024 * 1024 // 101 MB, > 100-MB-Limit

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
		t.Skip("erzeugt >100 MB Testdaten, siehe task test:unit")
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

// TestParse_XXEIsNotResolved schreibt fest, dass externe Entities nicht
// aufgelöst werden. encoding/xml unterstützt grundsätzlich keine
// DTD-Auflösung — ein Dokument mit einer nicht vordefinierten Entity wird
// komplett abgelehnt, statt die Entity stillschweigend zu ignorieren oder
// gar aufzulösen. Dieser Test hält beide Eigenschaften fest, falls sich
// das Verhalten der Standardbibliothek jemals ändert
// (IMPLEMENTIERUNG.md Abschnitt 12.3).
func TestParse_XXEIsNotResolved(t *testing.T) {
	t.Parallel()

	secretFile := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("geheim"), 0o600))

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

	require.Error(t, err, "ein Dokument mit einer externen Entity muss abgelehnt werden, nicht still verarbeitet")
	require.Nil(t, r)
	require.NotContains(t, err.Error(), "geheim",
		"der geheime Dateiinhalt darf nicht einmal in der Fehlermeldung auftauchen")
}
