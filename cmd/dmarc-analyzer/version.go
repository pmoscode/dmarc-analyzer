package main

import "runtime/debug"

// resolvedCommit liefert den Git-Commit des laufenden Builds: bevorzugt
// den per -ldflags gesetzten Wert (commit in main.go), sonst die von
// "go build" selbst eingebettete Revision (vcs.revision, nur vorhanden,
// wenn im Git-Checkout gebaut wurde — im Docker-Build fehlt .git, siehe
// .dockerignore, deshalb dort das Build-Arg COMMIT). Ein Build aus einem
// veränderten Arbeitsverzeichnis bekommt das Suffix "-dirty". Leer, wenn
// nichts davon verfügbar ist (z. B. "go run", "go test").
func resolvedCommit() string {
	if commit != "" {
		return commit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return commitFromBuildSettings(info.Settings)
}

// commitFromBuildSettings liest vcs.revision/vcs.modified aus den
// Build-Einstellungen — getrennt von resolvedCommit, damit es sich ohne
// echte Build-Informationen testen lässt.
func commitFromBuildSettings(settings []debug.BuildSetting) string {
	var revision string
	var modified bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision != "" && modified {
		return revision + "-dirty"
	}
	return revision
}
