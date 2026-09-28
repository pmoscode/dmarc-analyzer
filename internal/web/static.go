package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
)

// staticFS returns the filesystem for /static/ — embedded, or read from
// disk in development mode.
func staticFS(dev bool) (fs.FS, error) {
	if dev {
		return os.DirFS(devStaticDir), nil
	}
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return nil, fmt.Errorf("could not open embedded static files: %w", err)
	}
	return sub, nil
}

// devStaticDir: see devTemplatesDir in views.go — same assumption (the
// working directory is the repository root).
const devStaticDir = "internal/web/static"

// staticHandler serves /static/ requests with moderate caching — the
// files are part of the binary and only change with a new program
// release, so a long cache lifetime is unproblematic; in development mode
// (files change constantly) nothing is cached.
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
