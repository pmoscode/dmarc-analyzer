package dmarcxml

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxDecompressedSize limits the size of an unpacked attachment (per file
// and in total for .zip) to guard against zip/gzip bombs (IMPLEMENTIERUNG.md
// section 16).
const maxDecompressedSize = 100 * 1024 * 1024 // 100 MB

// ErrAttachmentTooLarge is returned when an unpacked attachment exceeds the
// size limit.
var ErrAttachmentTooLarge = errors.New("unpacked attachment exceeds the 100 mb size limit")

// errZipHasNoXML is returned when a zip archive contains no .xml file at
// all.
var errZipHasNoXML = errors.New("zip contains no .xml file")

// extractXML returns the XML documents contained in the attachment: exactly
// one for plain XML or .gz, all contained .xml files for .zip — some
// providers deliver multiple reports in one archive.
func extractXML(data []byte) ([][]byte, error) {
	switch {
	case isGzip(data):
		xmlData, err := decompressGzip(data)
		if err != nil {
			return nil, err
		}
		return [][]byte{xmlData}, nil
	case isZip(data):
		return decompressZip(data)
	default:
		// Plain .xml — the calling parser decides via Supports()/Parse()
		// whether it is actually a DMARC report.
		return [][]byte{data}, nil
	}
}

// isGzip detects gzip by the magic bytes 0x1f 0x8b.
func isGzip(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// isZip detects zip by the "PK" signature (local file header, central
// directory, or empty archive).
func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' &&
		(data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07)
}

func decompressGzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	out, err := readLimited(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	return out, nil
}

func decompressZip(data []byte) ([][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	var results [][]byte
	var total int64

	for _, f := range reader.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			continue
		}

		out, err := readZipEntry(f)
		if err != nil {
			return nil, err
		}

		total += int64(len(out))
		if total > maxDecompressedSize {
			return nil, ErrAttachmentTooLarge
		}

		results = append(results, out)
	}

	if len(results) == 0 {
		return nil, errZipHasNoXML
	}

	return results, nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open zip entry %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	out, err := readLimited(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip entry %q: %w", f.Name, err)
	}
	return out, nil
}

// readLimited reads at most maxDecompressedSize+1 bytes and reports
// ErrAttachmentTooLarge as soon as the limit is exceeded — without ever
// actually holding more than the limit in memory.
func readLimited(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, maxDecompressedSize+1)
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(out) > maxDecompressedSize {
		return nil, ErrAttachmentTooLarge
	}
	return out, nil
}

// looksLikeXML detects XML content by its start, for attachments without a
// meaningful filename.
func looksLikeXML(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<feedback"))
}
