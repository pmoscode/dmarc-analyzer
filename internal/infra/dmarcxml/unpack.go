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

// maxDecompressedSize begrenzt die Größe eines entpackten Anhangs (pro
// Datei und in Summe bei .zip) und schützt so vor Zip-/Gzip-Bomben
// (IMPLEMENTIERUNG.md Abschnitt 16).
const maxDecompressedSize = 100 * 1024 * 1024 // 100 MB

// ErrAttachmentTooLarge wird zurückgegeben, wenn ein entpackter Anhang das
// Größenlimit überschreitet.
var ErrAttachmentTooLarge = errors.New("entpackter anhang überschreitet das größenlimit von 100 mb")

// errZipHasNoXML wird zurückgegeben, wenn ein Zip-Archiv keine einzige
// .xml-Datei enthält.
var errZipHasNoXML = errors.New("zip enthält keine .xml-datei")

// extractXML liefert die im Anhang enthaltenen XML-Dokumente: bei blankem
// XML oder .gz genau eines, bei .zip alle enthaltenen .xml-Dateien —
// manche Provider liefern mehrere Reports in einem Archiv.
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
		// Blankes .xml — der aufrufende Parser entscheidet über
		// Supports()/Parse(), ob es sich tatsächlich um einen
		// DMARC-Report handelt.
		return [][]byte{data}, nil
	}
}

// isGzip erkennt Gzip anhand der Magic Bytes 0x1f 0x8b.
func isGzip(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// isZip erkennt Zip anhand der "PK"-Signatur (lokaler Dateikopf, zentrales
// Verzeichnis oder leeres Archiv).
func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' &&
		(data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07)
}

func decompressGzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip konnte nicht geöffnet werden: %w", err)
	}
	defer func() { _ = gz.Close() }()

	out, err := readLimited(gz)
	if err != nil {
		return nil, fmt.Errorf("gzip konnte nicht gelesen werden: %w", err)
	}
	return out, nil
}

func decompressZip(data []byte) ([][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip konnte nicht geöffnet werden: %w", err)
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
		return nil, fmt.Errorf("zip-eintrag %q konnte nicht geöffnet werden: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	out, err := readLimited(rc)
	if err != nil {
		return nil, fmt.Errorf("zip-eintrag %q konnte nicht gelesen werden: %w", f.Name, err)
	}
	return out, nil
}

// readLimited liest höchstens maxDecompressedSize+1 Bytes und meldet
// ErrAttachmentTooLarge, sobald die Grenze überschritten wird — ohne
// jemals mehr als das Limit tatsächlich im Speicher zu halten.
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

// looksLikeXML erkennt XML-Inhalt anhand des Anfangs, für Anhänge ohne
// aussagekräftigen Dateinamen.
func looksLikeXML(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<feedback"))
}
