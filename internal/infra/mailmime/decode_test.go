package mailmime_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/emersion/go-message"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/mailmime"
)

// buildMultipartMessage baut eine multipart/mixed-Testnachricht: ein
// Textkörper (kein Anhang) plus die übergebenen Anhänge.
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
	_, err = io.WriteString(bodyPart, "Anbei Ihr DMARC-Report.")
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

	// Manche Provider schicken den Report ohne umgebendes multipart/mixed
	// direkt als einzige Entity mit Content-Disposition: attachment.
	raw := []byte("Content-Type: application/gzip\r\n" +
		"Content-Disposition: attachment; filename=\"report.xml.gz\"\r\n" +
		"\r\n" +
		"binärer-inhalt-hier")

	attachments, err := mailmime.Decode(raw)
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, "report.xml.gz", attachments[0].Filename)
}

func TestDecode_FilenameFromContentTypeNameParam(t *testing.T) {
	t.Parallel()

	// Älterer, aber verbreiteter Weg: "name" im Content-Type statt
	// "filename" im Content-Disposition.
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

	// encoding/textproto ist tolerant gegenüber vielem; entscheidend ist,
	// dass Decode niemals abstürzt — ob es dabei einen Fehler oder ein
	// leeres Ergebnis liefert, ist beides akzeptabel (kein require auf
	// err nötig, ein Panic ließe den Test ohnehin fehlschlagen).
	require.NotPanics(t, func() {
		_, _ = mailmime.Decode([]byte("das ist keine gültige mime-nachricht \x00\x01\x02"))
	})
}
