package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
)

// staticFS liefert das Dateisystem für /static/ — eingebettet, oder im
// Entwicklungsmodus von der Festplatte.
func staticFS(dev bool) (fs.FS, error) {
	if dev {
		return os.DirFS(devStaticDir), nil
	}
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return nil, fmt.Errorf("eingebettete statische dateien konnten nicht geöffnet werden: %w", err)
	}
	return sub, nil
}

// devStaticDir: siehe devTemplatesDir in views.go — dieselbe Annahme
// (Arbeitsverzeichnis ist die Repository-Wurzel).
const devStaticDir = "internal/web/static"

// staticHandler liefert /static/-Anfragen mit moderatem Caching — die
// Dateien sind Teil der Binärdatei und ändern sich nur mit einem neuen
// Programm-Release, ein langes Cache-Alter ist deshalb unproblematisch;
// im Entwicklungsmodus (Dateien ändern sich laufend) wird nicht gecacht.
func (s *Server) staticHandler() http.Handler {
	fileServer := http.FileServerFS(s.staticFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.devMode {
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}
