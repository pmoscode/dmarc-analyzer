package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// sessionTTL ist die feste Gültigkeitsdauer einer Sitzung nach der
// Authentik-Anmeldung — kein Refresh, kein "angemeldet bleiben": nach
// Ablauf meldet sich ein Admin erneut über /anmelden an (i. d. R.
// transparent, weil Authentiks eigene SSO-Sitzung meist noch gültig ist).
const sessionTTL = 12 * time.Hour

// pendingLoginTTL begrenzt, wie lange zwischen /anmelden (Redirect zu
// Authentik) und /anmelden/callback vergehen darf — schützt vor einem
// state/nonce-Wert, der beliebig lange wiederverwendet werden könnte.
const pendingLoginTTL = 10 * time.Minute

// sessionCookieName und pendingLoginCookieName sind die Namen der beiden
// Cookies: eines für eine abgeschlossene Anmeldung (Sitzung), eines nur
// während des OIDC-Umwegs über Authentik (siehe handlers_login.go).
const (
	sessionCookieName      = "dmarc_session"
	pendingLoginCookieName = "dmarc_login"
)

// session ist eine abgeschlossene Anmeldung — mehrere gleichzeitig
// möglich (verschiedene Admins/Browser), anders als das frühere
// Ein-Sitzung-Modell der Desktop-Ära.
type session struct {
	username  string
	csrfToken string
	expiresAt time.Time
}

// pendingLogin hält state/nonce/PKCE-Verifier eines laufenden
// OIDC-Anmeldevorgangs (siehe handlers_login.go handleLoginStart/
// handleLoginCallback) — kurzlebig, ein Eintrag pro noch nicht
// abgeschlossenem Redirect zu Authentik.
type pendingLogin struct {
	state        string
	nonce        string
	pkceVerifier string
	expiresAt    time.Time
}

// auth verwaltet alle aktiven Sitzungen und laufenden Anmeldevorgänge
// dieses Serverlaufs.
type auth struct {
	mu       sync.Mutex
	sessions map[string]session
	pending  map[string]pendingLogin
}

func newAuth() *auth {
	return &auth{
		sessions: make(map[string]session),
		pending:  make(map[string]pendingLogin),
	}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("zufallswert konnte nicht erzeugt werden: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// beginLogin erzeugt einen neuen, pendingLoginTTL gültigen Anmeldevorgang
// und liefert dessen ID (Cookie-Wert) sowie state/nonce/PKCE-Verifier für
// die Authorization-Request an Authentik.
func (a *auth) beginLogin() (id string, p pendingLogin, err error) {
	id, err = randomHex(16)
	if err != nil {
		return "", pendingLogin{}, err
	}
	state, err := randomHex(16)
	if err != nil {
		return "", pendingLogin{}, err
	}
	nonce, err := randomHex(16)
	if err != nil {
		return "", pendingLogin{}, err
	}
	verifier, err := randomHex(32)
	if err != nil {
		return "", pendingLogin{}, err
	}

	p = pendingLogin{state: state, nonce: nonce, pkceVerifier: verifier, expiresAt: time.Now().Add(pendingLoginTTL)}

	a.mu.Lock()
	a.pending[id] = p
	a.mu.Unlock()

	return id, p, nil
}

// redeemLogin prüft id+state gegen einen ausstehenden Anmeldevorgang und
// verwirft ihn in jedem Fall — bei Erfolg wie bei Fehlschlag —, damit ein
// Callback nie zweimal eingelöst werden kann.
func (a *auth) redeemLogin(id, state string) (pendingLogin, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	p, ok := a.pending[id]
	delete(a.pending, id)

	if !ok || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(p.state)) != 1 {
		return pendingLogin{}, false
	}
	if time.Now().After(p.expiresAt) {
		return pendingLogin{}, false
	}
	return p, true
}

// createSession legt nach erfolgreicher OIDC-Anmeldung eine neue Sitzung
// für username an und liefert Cookie-Wert und CSRF-Token.
func (a *auth) createSession(username string) (cookieValue, csrfToken string, err error) {
	cookieValue, err = randomHex(32)
	if err != nil {
		return "", "", err
	}
	csrfToken, err = randomHex(32)
	if err != nil {
		return "", "", err
	}

	a.mu.Lock()
	a.sessions[cookieValue] = session{username: username, csrfToken: csrfToken, expiresAt: time.Now().Add(sessionTTL)}
	a.mu.Unlock()

	return cookieValue, csrfToken, nil
}

// sessionFromRequest liefert die Sitzung von r, falls das Cookie ein noch
// gültiges Sitzungs-Cookie ist — abgelaufene Einträge werden dabei
// gleich entfernt.
func (a *auth) sessionFromRequest(r *http.Request) (session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return session{}, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	s, ok := a.sessions[cookie.Value]
	if !ok {
		return session{}, false
	}
	if time.Now().After(s.expiresAt) {
		delete(a.sessions, cookie.Value)
		return session{}, false
	}
	return s, true
}

// validSession meldet, ob r ein gültiges Sitzungs-Cookie mitbringt.
func (a *auth) validSession(r *http.Request) bool {
	_, ok := a.sessionFromRequest(r)
	return ok
}

// csrfTokenForRequest liefert das CSRF-Token der Sitzung von r, oder einen
// leeren String ohne gültige Sitzung — für views.go beim Rendern eines
// Formulars.
func (a *auth) csrfTokenForRequest(r *http.Request) string {
	s, _ := a.sessionFromRequest(r)
	return s.csrfToken
}

// validCSRFToken meldet, ob token mit dem CSRF-Token der Sitzung von r
// übereinstimmt (siehe middleware.go requireCSRF).
func (a *auth) validCSRFToken(r *http.Request, token string) bool {
	s, ok := a.sessionFromRequest(r)
	return ok && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.csrfToken)) == 1
}

// endSession entfernt die Sitzung von r, falls vorhanden — für /abmelden.
func (a *auth) endSession(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	a.mu.Lock()
	delete(a.sessions, cookie.Value)
	a.mu.Unlock()
}

// setSessionCookie setzt das Sitzungs-Cookie nach erfolgreicher
// Authentik-Anmeldung. Secure+HttpOnly+SameSite=Lax: Lax statt Strict,
// weil der Browser dieses Cookie beim zurückkehrenden Redirect von
// Authentik (einem Cross-Site-Navigationsziel) mitschicken muss, damit
// /anmelden/callback die Sitzung setzen kann; ein direkter Angreifer kann
// über SameSite=Lax weiterhin keine zustandsändernde Anfrage auslösen
// (siehe requireCSRF zusätzlich dazu).
func setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// clearSessionCookie löscht das Sitzungs-Cookie beim Browser (Ablaufzeit
// in der Vergangenheit) — für /abmelden.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// setPendingLoginCookie/clearPendingLoginCookie verwalten das kurzlebige
// Cookie, das die ID des laufenden OIDC-Anmeldevorgangs zwischen
// /anmelden und /anmelden/callback transportiert.
func setPendingLoginCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     pendingLoginCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(pendingLoginTTL.Seconds()),
	})
}

func clearPendingLoginCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     pendingLoginCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
