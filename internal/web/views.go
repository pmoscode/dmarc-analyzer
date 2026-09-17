package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/pmoscode/dmarc-analyzer/internal/web/glossary"
)

// devTemplatesDir ist der Pfad, unter dem --entwicklung Vorlagen von der
// Festplatte liest — relativ zum Arbeitsverzeichnis, das beim Starten des
// Programms das Repository-Wurzelverzeichnis sein muss (z. B. `task run`).
const devTemplatesDir = "internal/web/templates"

// templateFuncs stellt Vorlagen-Hilfsfunktionen bereit: "glossarLink"
// wandelt einen Glossar-Begriffsnamen in einen Link auf die passende
// Erklärung auf /glossar um (MIGRATIONSPLAN.md Meilenstein M2:
// "Begriffs-Tooltips"), "csrfToken" liefert das CSRF-Token dieser
// Sitzung für versteckte Formularfelder (Meilenstein M3, siehe
// middleware.go requireCSRF) — als Funktion statt als Feld auf jeder
// einzelnen Seiten-Datenstruktur, weil sonst jede der inzwischen
// zahlreichen page-data-Structs (dashboardPageData, reportsPageData, …)
// dieses eine Feld einzeln bräuchte, nur weil layout.html jetzt überall
// ein Sync-Formular zeigt.
func newTemplateFuncs(csrfToken func() string) template.FuncMap {
	return template.FuncMap{
		"glossarLink": func(term string) string { return "/glossar#" + glossary.Slug(term) },
		"csrfToken":   csrfToken,
	}
}

// views lädt und rendert HTML-Vorlagen. Jede Seite (pages/*.html) wird
// zusammen mit layout.html zu einem eigenen *template.Template geparst —
// bewusst nicht alle Seiten in einem gemeinsamen Baum: Go-Templates
// teilen sich benannte Blöcke ({{define "content"}}) über alle mit
// ParseFS/ParseGlob gemeinsam geparsten Dateien hinweg; mehrere Seiten mit
// je einem eigenen "content"-Block in einem Baum würden sich gegenseitig
// überschreiben (letzte gewinnt), nicht wie ein Aufrufer erwarten würde.
type views struct {
	dev   bool
	funcs template.FuncMap

	mu    sync.RWMutex
	pages map[string]*template.Template
}

func newViews(dev bool, csrfToken func() string) (*views, error) {
	v := &views{dev: dev, funcs: newTemplateFuncs(csrfToken)}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *views) load() error {
	tmplFS, err := templatesFS(v.dev)
	if err != nil {
		return err
	}

	pageFiles, err := fs.Glob(tmplFS, "pages/*.html")
	if err != nil {
		return fmt.Errorf("seiten-vorlagen konnten nicht aufgelistet werden: %w", err)
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pf := range pageFiles {
		name := path.Base(pf)
		t, err := template.New("layout.html").Funcs(v.funcs).ParseFS(tmplFS, "layout.html", pf)
		if err != nil {
			return fmt.Errorf("vorlage %q konnte nicht geparst werden: %w", name, err)
		}
		pages[name] = t
	}

	v.mu.Lock()
	v.pages = pages
	v.mu.Unlock()
	return nil
}

func templatesFS(dev bool) (fs.FS, error) {
	if dev {
		return os.DirFS(devTemplatesDir), nil
	}
	return fs.Sub(embeddedTemplates, "templates")
}

// render führt die Vorlage page (z. B. "dashboard.html") gegen data aus
// und schreibt das Ergebnis nach w. Im Entwicklungsmodus wird vor jedem
// Aufruf neu von der Festplatte geladen, damit Änderungen ohne Neubau
// sichtbar werden (MIGRATIONSPLAN.md Abschnitt 6).
func (v *views) render(w http.ResponseWriter, page string, data any) error {
	return v.renderNamed(w, page, "layout.html", data)
}

// renderNamed führt einen benannten Block innerhalb der Vorlage page aus
// statt immer "layout.html" — Grundlage für htmx-Teilaktualisierungen
// (MIGRATIONSPLAN.md Abschnitt 7 "GET /berichte/seite"): dieselbe Datei
// (z. B. "reports.html") definiert per {{define "rows"}}...{{end}} einen
// Block, den sowohl der volle Seitenaufruf (über "content" eingebunden)
// als auch der htmx-Ladeknopf (direkt als "rows") ausführen können, ohne
// die Zeilen-Vorlage doppelt zu pflegen.
func (v *views) renderNamed(w http.ResponseWriter, page, tmplName string, data any) error {
	if v.dev {
		if err := v.load(); err != nil {
			return err
		}
	}

	v.mu.RLock()
	t, ok := v.pages[page]
	v.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unbekannte vorlage %q", page)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, tmplName, data); err != nil {
		return fmt.Errorf("vorlage %q (%q) konnte nicht gerendert werden: %w", page, tmplName, err)
	}
	return nil
}
