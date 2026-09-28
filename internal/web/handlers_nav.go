package web

// navItem is an entry in the main navigation (layout.html) — Active is
// set server-side based on the requested path (no JavaScript needed,
// works even with JS disabled).
type navItem struct {
	Label  string
	Href   string
	Active bool
}

// navItems builds the main navigation for the current request. Other
// filter parameters (zeitraum/domain) are deliberately not carried along —
// navigating via the nav is still, server-side, an ordinary GET to the
// plain page URL; that the same view still finds its last-used filter
// anyway is handled purely client-side by internal/web/static/app.js
// (localStorage per page path, no server state).
func navItems(currentPath string) []navItem {
	items := []struct{ label, href string }{
		{"Overview", "/"},
		{"Reports", "/berichte"},
		{"Sending sources", "/quellen"},
		{"Domains", "/domains"},
		{"Failures", "/fehlschlaege"},
		{"Import", "/import"},
		{"Settings", "/einstellungen"},
	}

	out := make([]navItem, len(items))
	for i, it := range items {
		out[i] = navItem{Label: it.label, Href: it.href, Active: it.href == currentPath}
	}
	return out
}
