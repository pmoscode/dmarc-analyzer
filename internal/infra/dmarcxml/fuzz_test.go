package dmarcxml_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
)

// FuzzParse stellt sicher, dass beliebiger Anhangsinhalt den Parser nie
// zum Absturz bringt (Panic) — Fehler sind erlaubt und erwartet, ein
// Absturz nicht (IMPLEMENTIERUNG.md Abschnitt 12.2: "Fuzzing auf dem
// XML-Parser"). Aufruf: go test -fuzz=FuzzParse ./internal/infra/dmarcxml
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
			f.Fatalf("seed %q konnte nicht gelesen werden: %v", path, err)
		}
		f.Add(data)
	}
	f.Add([]byte(""))
	f.Add([]byte("<feedback>"))
	f.Add([]byte("nicht einmal xml"))

	p := dmarcxml.NewParser()

	f.Fuzz(func(t *testing.T, data []byte) {
		attachment := sync.RawAttachment{Filename: "fuzz.xml", Data: data}
		// Weder Supports noch Parse dürfen bei beliebigem Input abstürzen.
		_ = p.Supports(attachment)
		_, _ = p.Parse(context.Background(), attachment)
	})
}
