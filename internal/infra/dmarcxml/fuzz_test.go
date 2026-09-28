package dmarcxml_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
)

// FuzzParse ensures that arbitrary attachment content never causes the
// parser to crash (panic) — errors are allowed and expected, a crash is not
// (IMPLEMENTIERUNG.md section 12.2: "fuzzing the XML parser"). Run with:
// go test -fuzz=FuzzParse ./internal/infra/dmarcxml
func FuzzParse(f *testing.F) {
	seedFiles := []string{
		filepath.Join("..", "..", "..", "testdata", "reports", "rfc7489", "sample.xml"),
		filepath.Join("..", "..", "..", "testdata", "reports", "quirky", "missing_pct.xml"),
		filepath.Join("..", "..", "..", "testdata", "reports", "quirky", "unknown_enums.xml"),
		filepath.Join("..", "..", "..", "testdata", "reports", "quirky", "zero_records.xml"),
	}
	for _, path := range seedFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("failed to read seed %q: %v", path, err)
		}
		f.Add(data)
	}
	f.Add([]byte(""))
	f.Add([]byte("<feedback>"))
	f.Add([]byte("not even xml"))

	p := dmarcxml.NewParser()

	f.Fuzz(func(t *testing.T, data []byte) {
		attachment := sync.RawAttachment{Filename: "fuzz.xml", Data: data}
		// Neither Supports nor Parse may crash on arbitrary input.
		_ = p.Supports(attachment)
		_, _ = p.Parse(context.Background(), attachment)
	})
}
