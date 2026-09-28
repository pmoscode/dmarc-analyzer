// Package mailmime breaks a raw email (RFC 5322/MIME) apart into its
// attachments. Protocol-independent: the same logic processes both IMAP
// messages (sync.RawMessage.Data) and imported .eml files — see the
// comment on sync.RawMessage.
package mailmime

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// Decoder implements sync.MessageDecoder against go-message. Stateless —
// a value is enough, no construction with dependencies needed.
type Decoder struct{}

var _ sync.MessageDecoder = Decoder{}

// NewDecoder creates a ready-to-use Decoder.
func NewDecoder() Decoder {
	return Decoder{}
}

// Decode satisfies sync.MessageDecoder via the package-wide Decode
// function.
func (Decoder) Decode(data []byte) ([]sync.RawAttachment, error) {
	return Decode(data)
}

// maxAttachmentSize limits the size of a single attachment being read —
// the same limit as unpacking in internal/infra/dmarcxml, applied here
// additionally before the actual unpacking
// (IMPLEMENTIERUNG.md section 16: zip/gzip bombs).
const maxAttachmentSize = 100 * 1024 * 1024 // 100 MB

// Decode reads a raw message and returns all attachments. Non-attachment
// parts (e.g. the text body "Here is your DMARC report") are skipped —
// ReportParser decides via Supports() anyway whether an attachment is a
// usable DMARC report, so dragging it along unnecessarily would be pure
// waste.
func Decode(data []byte) ([]sync.RawAttachment, error) {
	entity, err := message.Read(bytes.NewReader(data))
	if err != nil && !message.IsUnknownCharset(err) {
		return nil, fmt.Errorf("failed to read message: %w", err)
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
				return fmt.Errorf("failed to read mime part: %w", err)
			}
			if err := collectAttachments(part, out); err != nil {
				return err
			}
		}
		return nil
	}

	filename, ok := attachmentFilename(entity)
	if !ok {
		return nil // not an attachment, e.g. the mail's text body — skip
	}

	contentType, _, _ := entity.Header.ContentType()

	limited := io.LimitReader(entity.Body, maxAttachmentSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("failed to read attachment %q: %w", filename, err)
	}
	if len(data) > maxAttachmentSize {
		return fmt.Errorf("attachment %q exceeds the 100 mb size limit", filename)
	}

	*out = append(*out, sync.RawAttachment{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	})
	return nil
}

// attachmentFilename returns the filename of an attachment, if the part is
// one. Providers name this inconsistently: Content-Disposition "filename"
// is the RFC 2183 standard way, Content-Type "name" is an older, still
// common addition.
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
