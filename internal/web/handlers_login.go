package web

import "net/http"

// handleLogin tauscht einen gültigen Einmal-Code gegen ein Sitzungs-Cookie
// und leitet auf die Übersicht weiter (MIGRATIONSPLAN.md Abschnitt 5 und
// 7). Bewusst kein 404/redirect bei ungültigem Code, sondern 401 mit
// Klartext-Hinweis — ein abgelaufener Link soll nicht wie eine defekte
// Anwendung wirken.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if !s.auth.redeemCode(code) {
		http.Error(w, "Anmeldelink ungültig oder abgelaufen — bitte das Programm erneut starten.", http.StatusUnauthorized)
		return
	}

	s.auth.setSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
