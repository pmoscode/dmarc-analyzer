package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
)

// maxImportFileSize limits a single uploaded file (MIGRATIONSPLAN.md
// section 5: "uploads: size limit via http.MaxBytesReader (suggestion:
// 50 MB per file)"). The 100 MB unpack limit for zip/gzip bombs from AP 1
// additionally applies inside the parser, independent of this cap on the
// compressed upload size.
const maxImportFileSize = 50 << 20

// maxImportRequestSize limits the entire multipart request (several
// files at once) — more generous than a single file, but not unlimited.
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

// handleImportForm shows the import page with a drag-and-drop area
// (MIGRATIONSPLAN.md extension 9.3/E-6: "file import via upload/drag &
// drop"). The result of a previous upload arrives as a query parameter
// after the redirect from handleImportSubmit — the same stateless flash
// mechanism as /einstellungen.
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
	if err := s.views.render(w, r, "import.html", data); err != nil {
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

// handleImportSubmit processes one or more uploaded files
// (MIGRATIONSPLAN.md section 7: "POST /import"). Every file goes through
// importfiles.UseCase.ImportData directly from memory into the import —
// no intermediate step via disk, unlike the CLI subcommand "import"
// (which works with file paths).
func (s *Server) handleImportSubmit(w http.ResponseWriter, r *http.Request) {
	// r.Body is already limited to maxImportRequestSize via
	// MaxBytesReader — so ParseMultipartForm never reads more than that,
	// regardless of the maxMemory value passed here (which only controls
	// when Go additionally spills to temp files instead of pure memory).
	r.Body = http.MaxBytesReader(w, r.Body, maxImportRequestSize)
	//nolint:gosec // G120: see the MaxBytesReader limit directly above.
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
			total.Errors = append(total.Errors, fmt.Errorf("file %q exceeds the maximum size", fh.Filename))
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
