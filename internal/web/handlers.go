package web

import (
	"encoding/json"
	"net/http"
)

// serverError loggt err und antwortet mit einer Klartext-Fehlermeldung
// (UMSETZUNGSPLAN.md-Konvention: keine technischen Details an den
// Nutzer/Browser durchreichen).
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("anfrage fehlgeschlagen", "path", r.URL.Path, "error", err)
	http.Error(w, "Daten konnten nicht geladen werden.", http.StatusInternalServerError)
}

// writeJSON schreibt v als JSON-Antwort. NaN/Inf in float64-Feldern würden
// json.Marshal mit einem Fehler scheitern lassen (encoding/json kennt
// keine Darstellung dafür) — das würde hier als serverError sichtbar,
// nicht als kaputtes JSON beim Browser ankommen.
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
