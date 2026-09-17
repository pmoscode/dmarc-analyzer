package web

import (
	"log/slog"
	"net/http"
)

// handleLoginStart leitet zur Authentik-Authorize-URL weiter und legt
// dafür einen neuen, kurzlebigen Anmeldevorgang an (state/nonce/PKCE,
// siehe auth.go beginLogin).
func (s *Server) handleLoginStart(w http.ResponseWriter, r *http.Request) {
	id, p, err := s.auth.beginLogin()
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	setPendingLoginCookie(w, id)
	http.Redirect(w, r, s.oidc.authCodeURL(p.state, p.nonce, p.pkceVerifier), http.StatusSeeOther)
}

// handleLoginCallback verarbeitet die Rückkehr von Authentik: Code gegen
// Tokens tauschen, ID-Token validieren, Admin-Gruppe prüfen, Sitzung
// anlegen. Ein ungültiger/abgelaufener Anmeldevorgang oder eine fehlende
// Admin-Gruppenmitgliedschaft führen zu einem klaren Fehler statt einer
// Sitzung — kein automatischer erneuter Redirect zu Authentik, das würde
// bei einer dauerhaft fehlenden Berechtigung zu einer Schleife führen.
func (s *Server) handleLoginCallback(w http.ResponseWriter, r *http.Request) {
	defer clearPendingLoginCookie(w)

	cookie, err := r.Cookie(pendingLoginCookieName)
	if err != nil {
		http.Error(w, "Anmeldevorgang nicht gefunden oder abgelaufen — bitte erneut über /anmelden starten.", http.StatusBadRequest)
		return
	}

	pending, ok := s.auth.redeemLogin(cookie.Value, r.URL.Query().Get("state"))
	if !ok {
		http.Error(w, "Anmeldevorgang ungültig oder abgelaufen — bitte erneut über /anmelden starten.", http.StatusBadRequest)
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		slog.Warn("oidc-anmeldung von authentik abgelehnt", "error", errParam, "description", r.URL.Query().Get("error_description"))
		http.Error(w, "Anmeldung abgelehnt: "+errParam, http.StatusForbidden)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Antwort von Authentik enthält keinen Code.", http.StatusBadRequest)
		return
	}

	claims, err := s.oidc.exchange(r.Context(), code, pending.pkceVerifier, pending.nonce)
	if err != nil {
		slog.Error("oidc-token-austausch fehlgeschlagen", "error", err)
		http.Error(w, "Anmeldung fehlgeschlagen.", http.StatusUnauthorized)
		return
	}

	if !claims.isAdmin(s.oidc.adminGroup) {
		slog.Warn("oidc-anmeldung ohne admin-gruppe abgelehnt", "subject", claims.Subject, "email", claims.Email)
		http.Error(w, "Zugriff verweigert — dieses Konto ist nicht Mitglied der berechtigten Gruppe.", http.StatusForbidden)
		return
	}

	username := claims.Email
	if username == "" {
		username = claims.Subject
	}
	cookieValue, _, err := s.auth.createSession(username)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	setSessionCookie(w, cookieValue)
	slog.Info("admin angemeldet", "email", claims.Email)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout beendet die Sitzung und leitet — falls Authentik einen
// end_session_endpoint bekanntgibt — dorthin weiter (RP-initiated
// Logout), sonst auf /anmelden.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.endSession(r)
	clearSessionCookie(w)

	if u := s.oidc.endSessionURL(); u != "" {
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/anmelden", http.StatusSeeOther)
}
