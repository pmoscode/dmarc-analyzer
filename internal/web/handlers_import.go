package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
)

// maxImportFileSize begrenzt eine einzelne hochgeladene Datei
// (MIGRATIONSPLAN.md Abschnitt 5: "Uploads: Größenbegrenzung per
// http.MaxBytesReader (Vorschlag: 50 MB je Datei)"). Das 100-MB-
// Entpacklimit für Zip-/Gzip-Bomben aus AP 1 greift innerhalb des
// Parsers zusätzlich, unabhängig von dieser Obergrenze für die
// komprimierte Upload-Größe.
const maxImportFileSize = 50 << 20

// maxImportRequestSize begrenzt die gesamte Multipart-Anfrage (mehrere
// Dateien auf einmal) — großzügiger als eine einzelne Datei, aber nicht
// unbegrenzt.
const maxImportRequestSize = 300 << 20

type importPageData struct {
	Title string
	Nav   []navItem

	Error     string
	HasResult bool
	New       int
	Skipped   int
	Failed    int
}

// handleImportForm zeigt die Import-Seite mit Drag-&-Drop-Bereich
// (MIGRATIONSPLAN.md Erweiterung 9.3/E-6: "Datei-Import per Upload/Drag
// & Drop"). Ergebnis eines vorherigen Uploads kommt als Query-Parameter
// nach dem Redirect von handleImportSubmit — derselbe zustandslose
// Flash-Mechanismus wie /einstellungen.
func (s *Server) handleImportForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data := importPageData{
		Title:     "Import",
		Nav:       navItems(r.URL.Path),
		Error:     q.Get("fehler"),
		HasResult: q.Has("neu"),
		New:       parseIntOrZero(q.Get("neu")),
		Skipped:   parseIntOrZero(q.Get("uebersprungen")),
		Failed:    parseIntOrZero(q.Get("fehlerhaft")),
	}
	if err := s.views.render(w, "import.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func parseIntOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func redirectToImportWithError(w http.ResponseWriter, r *http.Request, message string) {
	v := url.Values{}
	v.Set("fehler", message)
	http.Redirect(w, r, "/import?"+v.Encode(), http.StatusSeeOther)
}

// handleImportSubmit verarbeitet einen oder mehrere hochgeladene Dateien
// (MIGRATIONSPLAN.md Abschnitt 7: "POST /import"). Jede Datei landet
// über importfiles.UseCase.ImportData direkt aus dem Speicher im Import
// — kein Zwischenschritt über die Festplatte, anders als der
// CLI-Unterbefehl "import" (der mit Dateipfaden arbeitet).
func (s *Server) handleImportSubmit(w http.ResponseWriter, r *http.Request) {
	// r.Body ist bereits über MaxBytesReader auf maxImportRequestSize
	// begrenzt — ParseMultipartForm liest also nie mehr als das,
	// unabhängig vom hier übergebenen maxMemory-Wert (der nur steuert, ab
	// wann Go zusätzlich auf temporäre Dateien statt reinen Speicher
	// ausweicht).
	r.Body = http.MaxBytesReader(w, r.Body, maxImportRequestSize)
	//nolint:gosec // G120: siehe MaxBytesReader-Begrenzung direkt darüber.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		redirectToImportWithError(w, r, "Die Datei(en) konnten nicht gelesen werden — insgesamt zu groß?")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	files := r.MultipartForm.File["dateien"]
	if len(files) == 0 {
		redirectToImportWithError(w, r, "Bitte mindestens eine Datei auswählen.")
		return
	}

	total := importfiles.Result{}
	for _, fh := range files {
		if fh.Size > maxImportFileSize {
			total.Failed++
			total.Errors = append(total.Errors, fmt.Errorf("datei %q überschreitet die maximale größe", fh.Filename))
			continue
		}

		f, err := fh.Open()
		if err != nil {
			total.Failed++
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, maxImportFileSize+1))
		_ = f.Close()
		if err != nil {
			total.Failed++
			continue
		}

		res, err := s.deps.Importer.ImportData(r.Context(), fh.Filename, data)
		if err != nil {
			total.Failed++
			continue
		}
		total.New += res.New
		total.Skipped += res.Skipped
		total.Failed += res.Failed
	}

	v := url.Values{}
	v.Set("neu", strconv.Itoa(total.New))
	v.Set("uebersprungen", strconv.Itoa(total.Skipped))
	v.Set("fehlerhaft", strconv.Itoa(total.Failed))
	http.Redirect(w, r, "/import?"+v.Encode(), http.StatusSeeOther)
}
