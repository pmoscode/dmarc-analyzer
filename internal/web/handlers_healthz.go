package web

import "net/http"

// handleHealthz ist der unauthentifizierte Healthcheck für Docker
// (HEALTHCHECK-Anweisung im Dockerfile) bzw. eine Orchestrierung
// (Liveness-/Readiness-Probe) — antwortet immer 200, solange der
// HTTP-Server selbst läuft. Kein Datenbankzugriff: ein einzelner
// langsamer Query soll den Container nicht fälschlich als "ungesund"
// markieren.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
