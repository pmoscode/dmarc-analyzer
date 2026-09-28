package mailmime_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/emersion/go-message"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/mailmime"
)

// buildMultipartMessage builds a multipart/mixed test message: a text body
// (not an attachment) plus the given attachments.
func buildMultipartMessage(t *testing.T, attachments map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	var h message.Header
	h.SetContentType("multipart/mixed", nil)
	w, err := message.CreateWriter(&buf, h)
	require.NoError(t, err)

	var bodyHeader message.Header
	bodyHeader.SetContentType("text/plain", nil)
	bodyPart, err := w.CreatePart(bodyHeader)
	require.NoError(t, err)
	_, err = io.WriteString(bodyPart, "Attached is your DMARC report.")
	require.NoError(t, err)
	require.NoError(t, bodyPart.Close())

	for filename, content := range attachments {
		var attHeader message.Header
		attHeader.SetContentType("application/octet-stream", nil)
		attHeader.SetContentDisposition("attachment", map[string]string{"filename": filename})
		part, err := w.CreatePart(attHeader)
		require.NoError(t, err)
		_, err = io.WriteString(part, content)
		require.NoError(t, err)
		require.NoError(t, part.Close())
	}

	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestDecode_MultipartWithOneAttachment(t *testing.T) {
	t.Parallel()

	raw := buildMultipartMessage(t, map[string]string{"report.xml": "<feedback></feedback>"})

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, "report.xml", attachments[0].Filename)
	require.Equal(t, "<feedback></feedback>", string(attachments[0].Data))
}

func TestDecode_MultipartWithMultipleAttachments(t *testing.T) {
	t.Parallel()

	raw := buildMultipartMessage(t, map[string]string{
		"report-a.xml": "A",
		"report-b.xml": "B",
	})

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Len(t, attachments, 2)

	names := map[string]string{}
	for _, a := range attachments {
		names[a.Filename] = string(a.Data)
	}
	require.Equal(t, "A", names["report-a.xml"])
	require.Equal(t, "B", names["report-b.xml"])
}

func TestDecode_TextBodyWithoutAttachment_ReturnsNoAttachments(t *testing.T) {
	t.Parallel()

	raw := buildMultipartMessage(t, nil)

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Empty(t, attachments)
}

func TestDecode_NonMultipartAttachmentOnlyMessage(t *testing.T) {
	t.Parallel()

	// Some providers send the report without an enclosing multipart/mixed,
	// directly as the sole entity with Content-Disposition: attachment.
	raw := []byte("Content-Type: application/gzip\r\n" +
		"Content-Disposition: attachment; filename=\"report.xml.gz\"\r\n" +
		"\r\n" +
		"binary-content-here")

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, "report.xml.gz", attachments[0].Filename)
}

func TestDecode_FilenameFromContentTypeNameParam(t *testing.T) {
	t.Parallel()

	// Older but still common approach: "name" in Content-Type instead of
	// "filename" in Content-Disposition.
	raw := []byte("Content-Type: application/xml; name=\"legacy-report.xml\"\r\n" +
		"\r\n" +
		"<feedback></feedback>")

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, "legacy-report.xml", attachments[0].Filename)
}

func TestDecode_InvalidMessage_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// encoding/textproto tolerates a lot; what matters is that Decode never
	// crashes — whether it returns an error or an empty result is both
	// acceptable (no require on err needed, a panic would fail the test
	// anyway).
	require.NotPanics(t, func() {
		_, _ = mailmime.Decode([]byte("this is not a valid mime message \x00\x01\x02"))
	})
}
