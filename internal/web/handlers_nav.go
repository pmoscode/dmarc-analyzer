package web

import "net/http"

// navItem ist ein Eintrag der Hauptnavigation (layout.html) — Active wird
// serverseitig anhand des angeforderten Pfads gesetzt (kein JavaScript
// nötig, funktioniert auch ohne aktiviertes JS).
type navItem struct {
	Label  string
	Href   string
	Active bool
}

// navItems baut die Hauptnavigation für die aktuelle Anfrage. Weitere
// Filterparameter (zeitraum/domain) werden bewusst nicht mitgegeben —
// ein Seitenwechsel über die Navigation beginnt wieder mit der
// Voreinstellung, das entspricht MIGRATIONSPLAN.md Abschnitt 8
// ("Filterleiste ... wirkt weiter auf Übersicht, Berichte, Quellen"),
// nicht "Filter über Seitenwechsel hinweg merken" (kein Vorgabe dafür).
func navItems(currentPath string) []navItem {
	items := []struct{ label, href string }{
		{"Übersicht", "/"},
		{"Berichte", "/berichte"},
		{"Sendequellen", "/quellen"},
		{"Glossar", "/glossar"},
		{"Einstellungen", "/einstellungen"},
	}

	out := make([]navItem, len(items))
	for i, it := range items {
		out[i] = navItem{Label: it.label, Href: it.href, Active: it.href == currentPath}
	}
	return out
}

// placeholderPageData rendert eine noch inhaltslose Seite (Meilenstein
// M1: "Navigation zwischen leeren Seiten") — Berichte, Sendequellen,
// Glossar und Einstellungen bekommen ihren eigentlichen Inhalt erst in
// M2/M3; bis dahin bestätigt diese Seite nur, dass Routing, Navigation
// und Theme bereits für die künftige Seite funktionieren.
type placeholderPageData struct {
	Title string
	Nav   []navItem
	Note  string
}

func (s *Server) handlePlaceholder(title, note string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := placeholderPageData{
			Title: title,
			Nav:   navItems(r.URL.Path),
			Note:  note,
		}
		if err := s.views.render(w, "placeholder.html", data); err != nil {
			s.serverError(w, r, err)
		}
	}
}
