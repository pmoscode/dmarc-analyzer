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

// devTemplatesDir is the path dev mode (DMARC_DEV_MODE=true) reads
// templates from on disk — relative to the working directory, which must
// be the repository root when the program starts (e.g. `task run`).
const devTemplatesDir = "internal/web/templates"

// newTemplateFuncs provides template helper functions: "glossaryDef"
// returns a glossary term's explanation directly as text — for the "?"
// hints that show the explanation inline (as a tooltip/popover) instead
// of via a link to a dedicated glossary page (there is no glossary page
// anymore, see formerly handlers_glossary.go). "csrfToken" returns THIS
// request's CSRF token (every session has had its own token since the
// move to OIDC multi-user logins, see auth.go) — registered as a
// placeholder function that render()/renderNamed() rebind to this
// request's actual value before every execution via Template.Funcs()
// (see render below). "buildInfo" returns the running build's version and
// git commit for the header (layout.html) — fixed for the entire runtime,
// so unlike "csrfToken" it's registered directly here.
func newTemplateFuncs(build BuildInfo) template.FuncMap {
	return template.FuncMap{
		"glossaryDef": func(term string) string {
			t, _ := glossary.ByName(term)
			return t.Definition
		},
		"csrfToken": func() string { return "" },
		"buildInfo": func() BuildInfo { return build },
	}
}

// views loads and renders HTML templates. Each page (pages/*.html) is
// parsed together with layout.html into its own *template.Template —
// deliberately not all pages in one shared tree: Go templates share named
// blocks ({{define "content"}}) across all files parsed together via
// ParseFS/ParseGlob; several pages each with their own "content" block in
// one tree would overwrite each other (last one wins), not what a caller
// would expect.
type views struct {
	dev   bool
	funcs template.FuncMap
	// csrfToken returns the CSRF token for the session of r — passed in
	// by the server (see the newViews call in server.go) so views doesn't
	// have to depend on auth.go itself.
	csrfToken func(*http.Request) string

	mu    sync.RWMutex
	pages map[string]*template.Template
}

func newViews(dev bool, build BuildInfo, csrfToken func(*http.Request) string) (*views, error) {
	v := &views{dev: dev, funcs: newTemplateFuncs(build), csrfToken: csrfToken}
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
		return fmt.Errorf("could not list page templates: %w", err)
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pf := range pageFiles {
		name := path.Base(pf)
		t, err := template.New("layout.html").Funcs(v.funcs).ParseFS(tmplFS, "layout.html", pf)
		if err != nil {
			return fmt.Errorf("could not parse template %q: %w", name, err)
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

// render executes the page template (e.g. "dashboard.html") against data
// and writes the result to w. In development mode it reloads from disk
// before every call, so changes are visible without a rebuild.
func (v *views) render(w http.ResponseWriter, r *http.Request, page string, data any) error {
	return v.renderNamed(w, r, page, "layout.html", data)
}

// renderNamed executes a named block within the page template instead of
// always "layout.html" — the basis for htmx partial updates
// (MIGRATIONSPLAN.md section 7 "GET /berichte/seite"): the same file
// (e.g. "reports.html") defines a block via {{define "rows"}}...{{end}}
// that both the full page view (included via "content") and the htmx
// load-more button (executed directly as "rows") can run, without
// maintaining the row template twice.
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
		return fmt.Errorf("unknown template %q", page)
	}

	// Clone + Funcs() instead of keeping the placeholder registered at
	// parse time: the CSRF token depends on the particular session
	// (several admins can be logged in at once, see auth.go), but t
	// itself is only parsed once and shared across requests (v.pages).
	// Clone() duplicates the whole template collection (layout + page)
	// cheaply enough for a low-throughput admin UI.
	token := v.csrfToken(r)
	cloned, err := t.Clone()
	if err != nil {
		return fmt.Errorf("could not prepare template %q for this request: %w", page, err)
	}
	cloned = cloned.Funcs(template.FuncMap{"csrfToken": func() string { return token }})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := cloned.ExecuteTemplate(w, tmplName, data); err != nil {
		return fmt.Errorf("could not render template %q (%q): %w", page, tmplName, err)
	}
	return nil
}
