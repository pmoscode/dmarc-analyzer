package web

import (
	"net/http"

	"github.com/pmoscode/dmarc-analyzer/internal/web/glossary"
)

// glossaryPageData rendert die Glossarseite (MIGRATIONSPLAN.md
// Meilenstein M2: "Glossarseite und Begriffs-Tooltips") — jeder Begriff
// bekommt einen Anker (Slug), auf den "?"-Links von anderen Seiten
// verweisen (siehe views.go templateFuncs["glossarLink"]).
type glossaryPageData struct {
	Title string
	Nav   []navItem
	Terms []glossaryTermView
}

type glossaryTermView struct {
	Slug       string
	Name       string
	Definition string
}

func (s *Server) handleGlossary(w http.ResponseWriter, r *http.Request) {
	terms := make([]glossaryTermView, len(glossary.Terms))
	for i, t := range glossary.Terms {
		terms[i] = glossaryTermView{Slug: glossary.Slug(t.Name), Name: t.Name, Definition: t.Definition}
	}

	data := glossaryPageData{
		Title: "Glossar",
		Nav:   navItems(r.URL.Path),
		Terms: terms,
	}
	if err := s.views.render(w, "glossary.html", data); err != nil {
		s.serverError(w, r, err)
	}
}
