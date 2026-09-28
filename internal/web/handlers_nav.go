package web

// navItem ist ein Eintrag der Hauptnavigation (layout.html) — Active wird
// serverseitig anhand des angeforderten Pfads gesetzt (kein JavaScript
// nötig, funktioniert auch ohne aktiviertes JS).
type navItem struct {
	Label  string
	Href   string
	Active bool
}

// navItems baut die Hauptnavigation für die aktuelle Anfrage. Weitere
// Filterparameter (zeitraum/domain) werden bewusst nicht mitgegeben — ein
// Seitenwechsel über die Navigation ist serverseitig weiterhin ein
// gewöhnlicher GET auf die reine Seiten-URL; dass dieselbe Ansicht ihren
// zuletzt benutzten Filter trotzdem wiederfindet, übernimmt rein
// client-seitig internal/web/static/app.js (localStorage je Seitenpfad,
// kein Server-Zustand).
func navItems(currentPath string) []navItem {
	items := []struct{ label, href string }{
		{"Übersicht", "/"},
		{"Berichte", "/berichte"},
		{"Sendequellen", "/quellen"},
		{"Domains", "/domains"},
		{"Fehlschläge", "/fehlschlaege"},
		{"Import", "/import"},
		{"Einstellungen", "/einstellungen"},
	}

	out := make([]navItem, len(items))
	for i, it := range items {
		out[i] = navItem{Label: it.label, Href: it.href, Active: it.href == currentPath}
	}
	return out
}
