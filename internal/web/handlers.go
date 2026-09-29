package web

import (
	"encoding/json"
	"net/http"
)

// serverError logs err and responds with a plaintext error message
// (UMSETZUNGSPLAN.md convention: don't pass technical details to the
// user/browser).
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "path", r.URL.Path, "error", err)
	http.Error(w, "Daten konnten nicht geladen werden.", http.StatusInternalServerError)
}

// writeJSON writes v as a JSON response. NaN/Inf in float64 fields would
// make json.Marshal fail with an error (encoding/json has no
// representation for them) — that surfaces here as serverError, not as
// broken JSON arriving at the browser.
func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
