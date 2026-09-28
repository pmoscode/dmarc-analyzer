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

// sessionTTL is the fixed validity period of a session after the
// Authentik login — no refresh, no "stay logged in": once it expires, an
// admin logs in again via /anmelden (usually transparent, since
// Authentik's own SSO session is typically still valid).
const sessionTTL = 12 * time.Hour

// pendingLoginTTL limits how long may pass between /anmelden (redirect to
// Authentik) and /anmelden/callback — protects against a state/nonce
// value that could otherwise be reused indefinitely.
const pendingLoginTTL = 10 * time.Minute

// sessionCookieName and pendingLoginCookieName are the names of the two
// cookies: one for a completed login (session), one only during the OIDC
// detour through Authentik (see handlers_login.go).
const (
	sessionCookieName      = "dmarc_session"
	pendingLoginCookieName = "dmarc_login"
)

// session is a completed login — several can exist at once (different
// admins/browsers), unlike the former single-session model from the
// desktop era.
type session struct {
	username  string
	csrfToken string
	expiresAt time.Time
}

// pendingLogin holds the state/nonce/PKCE verifier of an in-progress OIDC
// login attempt (see handlers_login.go handleLoginStart/
// handleLoginCallback) — short-lived, one entry per redirect to Authentik
// that hasn't completed yet.
type pendingLogin struct {
	state        string
	nonce        string
	pkceVerifier string
	expiresAt    time.Time
}

// auth manages all active sessions and in-progress login attempts for
// this server run.
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
		return "", fmt.Errorf("could not generate random value: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// beginLogin creates a new login attempt valid for pendingLoginTTL and
// returns its ID (cookie value) plus state/nonce/PKCE verifier for the
// authorization request to Authentik.
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

// redeemLogin checks id+state against a pending login attempt and
// discards it either way — on success as well as on failure — so a
// callback can never be redeemed twice.
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

// createSession creates a new session for username after a successful
// OIDC login and returns the cookie value and CSRF token.
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

// sessionFromRequest returns the session from r if the cookie is still a
// valid session cookie — expired entries are removed along the way.
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

// validSession reports whether r carries a valid session cookie.
func (a *auth) validSession(r *http.Request) bool {
	_, ok := a.sessionFromRequest(r)
	return ok
}

// csrfTokenForRequest returns the CSRF token of the session from r, or an
// empty string without a valid session — used by views.go when rendering
// a form.
func (a *auth) csrfTokenForRequest(r *http.Request) string {
	s, _ := a.sessionFromRequest(r)
	return s.csrfToken
}

// validCSRFToken reports whether token matches the CSRF token of the
// session from r (see middleware.go requireCSRF).
func (a *auth) validCSRFToken(r *http.Request, token string) bool {
	s, ok := a.sessionFromRequest(r)
	return ok && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.csrfToken)) == 1
}

// endSession removes the session from r, if any — for /abmelden.
func (a *auth) endSession(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	a.mu.Lock()
	delete(a.sessions, cookie.Value)
	a.mu.Unlock()
}

// setSessionCookie sets the session cookie after a successful Authentik
// login. Secure+HttpOnly+SameSite=Lax: Lax instead of Strict, because the
// browser must send this cookie on the returning redirect from Authentik
// (a cross-site navigation target) so that /anmelden/callback can set the
// session; a direct attacker still can't trigger a state-changing request
// via SameSite=Lax (see requireCSRF in addition to this).
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

// clearSessionCookie deletes the session cookie on the browser (expiry in
// the past) — for /abmelden.
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

// setPendingLoginCookie/clearPendingLoginCookie manage the short-lived
// cookie that carries the ID of the in-progress OIDC login attempt
// between /anmelden and /anmelden/callback.
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
