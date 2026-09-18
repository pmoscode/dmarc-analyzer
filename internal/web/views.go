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

// devTemplatesDir ist der Pfad, unter dem Dev-Modus (DMARC_DEV_MODE=true)
// Vorlagen von der Festplatte liest — relativ zum Arbeitsverzeichnis, das
// beim Starten des Programms das Repository-Wurzelverzeichnis sein muss
// (z. B. `task run`).
const devTemplatesDir = "internal/web/templates"

// templateFuncs stellt Vorlagen-Hilfsfunktionen bereit: "glossaryDef"
// liefert die Erklärung eines Glossar-Begriffs direkt als Text — für die
// "?"-Hinweise, die die Erklärung inline (als Tooltip/Popover) statt über
// einen Link auf eine eigene Glossarseite anzeigen (es gibt keine
// Glossarseite mehr, siehe ehemals handlers_glossary.go). "csrfToken"
// liefert das CSRF-Token DIESER Anfrage (jede Sitzung hat seit dem
// Umstieg auf OIDC-Mehrbenutzer-Logins ihr eigenes Token, siehe auth.go)
// — als Platzhalter-Funktion registriert, die render()/renderNamed() vor
// jeder Ausführung per Template.Funcs() auf den tatsächlichen Wert dieser
// Anfrage umbiegen (siehe render unten).
func newTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"glossaryDef": func(term string) string {
			t, _ := glossary.ByName(term)
			return t.Definition
		},
		"csrfToken": func() string { return "" },
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
	// csrfToken liefert das CSRF-Token für die Sitzung von r — vom Server
	// übergeben (siehe newViews-Aufruf in server.go), damit views nicht
	// selbst von auth.go abhängen muss.
	csrfToken func(*http.Request) string

	mu    sync.RWMutex
	pages map[string]*template.Template
}

func newViews(dev bool, csrfToken func(*http.Request) string) (*views, error) {
	v := &views{dev: dev, funcs: newTemplateFuncs(), csrfToken: csrfToken}
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
// sichtbar werden.
func (v *views) render(w http.ResponseWriter, r *http.Request, page string, data any) error {
	return v.renderNamed(w, r, page, "layout.html", data)
}

// renderNamed führt einen benannten Block innerhalb der Vorlage page aus
// statt immer "layout.html" — Grundlage für htmx-Teilaktualisierungen
// (MIGRATIONSPLAN.md Abschnitt 7 "GET /berichte/seite"): dieselbe Datei
// (z. B. "reports.html") definiert per {{define "rows"}}...{{end}} einen
// Block, den sowohl der volle Seitenaufruf (über "content" eingebunden)
// als auch der htmx-Ladeknopf (direkt als "rows") ausführen können, ohne
// die Zeilen-Vorlage doppelt zu pflegen.
func (v *views) renderNamed(w http.ResponseWriter, r *http.Request, page, tmplName string, data any) error {
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

	// Clone + Funcs() statt den zur Parse-Zeit registrierten Platzhalter
	// zu behalten: das CSRF-Token hängt von der jeweiligen Sitzung ab
	// (mehrere gleichzeitig angemeldete Admins, siehe auth.go), t selbst
	// wird aber nur einmal geparst und zwischen Anfragen geteilt
	// (v.pages). Clone() dupliziert die gesamte Vorlagensammlung (layout +
	// Seite) günstig genug für eine Admin-Oberfläche ohne hohen Durchsatz.
	token := v.csrfToken(r)
	cloned, err := t.Clone()
	if err != nil {
		return fmt.Errorf("vorlage %q konnte nicht für diese anfrage vorbereitet werden: %w", page, err)
	}
	cloned = cloned.Funcs(template.FuncMap{"csrfToken": func() string { return token }})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := cloned.ExecuteTemplate(w, tmplName, data); err != nil {
		return fmt.Errorf("vorlage %q (%q) konnte nicht gerendert werden: %w", page, tmplName, err)
	}
	return nil
}
