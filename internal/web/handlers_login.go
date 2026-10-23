package web

import (
	"log/slog"
	"net/http"
)

// handleLoginStart redirects to the Authentik authorize URL and creates a
// new, short-lived login attempt for it (state/nonce/PKCE, see auth.go
// beginLogin).
func (s *Server) handleLoginStart(w http.ResponseWriter, r *http.Request) {
	authenticator, ok := s.oidcOrUnavailable(w, r)
	if !ok {
		return
	}

	id, p, err := s.auth.beginLogin()
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	setPendingLoginCookie(w, id)
	//nolint:gosec // G710: false positive — the target is Authentik's
	// authorize endpoint from discovery (DMARC_OIDC_ISSUER_URL); only
	// r.Context() flows into the lookup, nothing from the request URL.
	http.Redirect(w, r, authenticator.authCodeURL(p.state, p.nonce, p.pkceVerifier), http.StatusSeeOther)
}

// handleLoginCallback processes the return from Authentik: exchange the
// code for tokens, validate the ID token, check the admin group, create a
// session. An invalid/expired login attempt or missing admin group
// membership results in a clear error instead of a session — no automatic
// redirect back to Authentik, since that would loop forever for a
// permanently missing authorization.
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
		slog.Warn("oidc login rejected by authentik", "error", errParam, "description", r.URL.Query().Get("error_description"))
		http.Error(w, "Anmeldung abgelehnt: "+errParam, http.StatusForbidden)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Antwort von Authentik enthält keinen Code.", http.StatusBadRequest)
		return
	}

	authenticator, ok := s.oidcOrUnavailable(w, r)
	if !ok {
		return
	}

	claims, err := authenticator.exchange(r.Context(), code, pending.pkceVerifier, pending.nonce)
	if err != nil {
		slog.Error("oidc token exchange failed", "error", err)
		http.Error(w, "Anmeldung fehlgeschlagen.", http.StatusUnauthorized)
		return
	}

	username := claims.Email
	if username == "" {
		username = claims.Subject
	}

	if !claims.isAdmin(authenticator.adminGroup) {
		slog.Warn("oidc login rejected: missing admin group", "subject", claims.Subject, "email", claims.Email)
		s.renderAccessDenied(w, r, username)
		return
	}

	cookieValue, _, err := s.auth.createSession(username)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	setSessionCookie(w, cookieValue)
	slog.Info("admin logged in", "email", claims.Email)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// accessDeniedData holds the values for access_denied.html — a standalone
// page without the app layout (see the comment there), since no session
// exists at this point.
type accessDeniedData struct {
	// Account is the email address or, if the ID token doesn't contain
	// one, the OIDC subject — the same fallback as for the session
	// username (see the caller).
	Account string
}

// renderAccessDenied shows an explanatory error page instead of a bare
// "403 Forbidden" plaintext response (the previous state, before anyone
// without the admin group had ever tried to log in — reproduced
// 2026-09-19: apart from that one sentence, the page was completely
// empty). Still renders correctly with http.StatusForbidden: Content-Type
// is set BEFORE WriteHeader; views.renderNamed sets it again, but that's a
// no-op after WriteHeader (already the correct value).
func (s *Server) renderAccessDenied(w http.ResponseWriter, r *http.Request, account string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	if err := s.views.renderNamed(w, r, "access_denied.html", "fullpage", accessDeniedData{Account: account}); err != nil {
		slog.Error("could not render access-denied page", "error", err)
	}
}

// handleLogout ends the session and — if Authentik advertises an
// end_session_endpoint — redirects there (RP-initiated logout), otherwise
// to /anmelden.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.endSession(r)
	clearSessionCookie(w)

	// With Authentik unreachable, the logout stays local.
	if authenticator, err := s.oidc.get(r.Context()); err == nil {
		if u := authenticator.endSessionURL(); u != "" {
			http.Redirect(w, r, u, http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/anmelden", http.StatusSeeOther)
}

// oidcOrUnavailable returns the discovered OIDC authenticator or — while
// Authentik is unreachable (see lazyOIDCAuthenticator) — answers 503 and
// returns false.
func (s *Server) oidcOrUnavailable(w http.ResponseWriter, r *http.Request) (*oidcAuthenticator, bool) {
	authenticator, err := s.oidc.get(r.Context())
	if err != nil {
		slog.Warn("oidc provider unavailable", "error", err)
		http.Error(w, "Anmeldung derzeit nicht möglich, weil Authentik nicht erreichbar ist — bitte in ein paar Minuten erneut versuchen.", http.StatusServiceUnavailable)
		return nil, false
	}
	return authenticator, true
}
