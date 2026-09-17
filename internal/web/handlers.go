package web

import (
	"encoding/json"
	"net/http"
	"unicode"
)

// serverError loggt err und antwortet mit einer Klartext-Fehlermeldung
// (UMSETZUNGSPLAN.md-Konvention: keine technischen Details an den
// Nutzer/Browser durchreichen).
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("anfrage fehlgeschlagen", "path", r.URL.Path, "error", err)
	http.Error(w, "Daten konnten nicht geladen werden.", http.StatusInternalServerError)
}

// displayError bereitet eine err.Error()-Meldung für die Anzeige im
// Formular auf: Go-Konvention verlangt kleingeschriebene, satzzeichenlose
// Fehlertexte (revive error-strings, siehe z. B.
// internal/domain/account.NewMailAccount) — für die direkte Anzeige im
// Browser (kein Log, kein %w-Wrapping mehr davor) wird daraus wieder ein
// normaler Satz.
func displayError(err error) string {
	msg := []rune(err.Error())
	if len(msg) == 0 {
		return ""
	}
	msg[0] = unicode.ToUpper(msg[0])
	return string(msg) + "."
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
