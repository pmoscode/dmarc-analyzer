// Package mailmime zerlegt eine rohe E-Mail (RFC 5322/MIME) in ihre
// Anhänge. Protokollunabhängig: dieselbe Logik verarbeitet sowohl
// IMAP-Nachrichten (sync.RawMessage.Data) als auch importierte
// .eml-Dateien — siehe Kommentar an sync.RawMessage.
package mailmime

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// Decoder implementiert sync.MessageDecoder gegen go-message. Zustandslos —
// ein Wert genügt, keine Konstruktion mit Abhängigkeiten nötig.
type Decoder struct{}

var _ sync.MessageDecoder = Decoder{}

// NewDecoder erzeugt einen einsatzbereiten Decoder.
func NewDecoder() Decoder {
	return Decoder{}
}

// Decode erfüllt sync.MessageDecoder über die paketweite Decode-Funktion.
func (Decoder) Decode(data []byte) ([]sync.RawAttachment, error) {
	return Decode(data)
}

// maxAttachmentSize begrenzt die Größe eines einzelnen gelesenen Anhangs —
// dieselbe Grenze wie beim Entpacken in internal/infra/dmarcxml, hier
// zusätzlich vor dem eigentlichen Entpacken angewendet
// (IMPLEMENTIERUNG.md Abschnitt 16: Zip-/Gzip-Bomben).
const maxAttachmentSize = 100 * 1024 * 1024 // 100 MB

// Decode liest eine rohe Nachricht und liefert alle Anhänge. Nicht-
// Anhang-Teile (z. B. der Textkörper "Hier ist Ihr DMARC-Report") werden
// übersprungen — ReportParser entscheidet ohnehin selbst per Supports(),
// ob ein Anhang ein verwertbarer DMARC-Report ist, das unnötig
// mitzuschleifen wäre reine Verschwendung.
func Decode(data []byte) ([]sync.RawAttachment, error) {
	entity, err := message.Read(bytes.NewReader(data))
	if err != nil && !message.IsUnknownCharset(err) {
		return nil, fmt.Errorf("nachricht konnte nicht gelesen werden: %w", err)
	}

	var attachments []sync.RawAttachment
	if err := collectAttachments(entity, &attachments); err != nil {
		return nil, err
	}
	return attachments, nil
}

func collectAttachments(entity *message.Entity, out *[]sync.RawAttachment) error {
	if mr := entity.MultipartReader(); mr != nil {
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("mime-teil konnte nicht gelesen werden: %w", err)
			}
			if err := collectAttachments(part, out); err != nil {
				return err
			}
		}
		return nil
	}

	filename, ok := attachmentFilename(entity)
	if !ok {
		return nil // kein Anhang, z. B. der Textkörper der Mail — überspringen
	}

	contentType, _, _ := entity.Header.ContentType()

	limited := io.LimitReader(entity.Body, maxAttachmentSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("anhang %q konnte nicht gelesen werden: %w", filename, err)
	}
	if len(data) > maxAttachmentSize {
		return fmt.Errorf("anhang %q überschreitet das größenlimit von 100 mb", filename)
	}

	*out = append(*out, sync.RawAttachment{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	})
	return nil
}

// attachmentFilename liefert den Dateinamen eines Anhangs, falls der Teil
// einer ist. Providers benennen das uneinheitlich: Content-Disposition
// "filename" ist der RFC-2183-Standardweg, Content-Type "name" ein
// älterer, weiterhin verbreiteter Zusatz.
func attachmentFilename(entity *message.Entity) (string, bool) {
	if _, params, err := entity.Header.ContentDisposition(); err == nil {
		if name := strings.TrimSpace(params["filename"]); name != "" {
			return name, true
		}
	}
	if _, params, err := entity.Header.ContentType(); err == nil {
		if name := strings.TrimSpace(params["name"]); name != "" {
			return name, true
		}
	}
	return "", false
}
