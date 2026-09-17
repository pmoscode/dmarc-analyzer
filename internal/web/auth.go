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

// oneTimeCodeTTL begrenzt die Gültigkeit eines Einmal-Anmeldelinks
// (MIGRATIONSPLAN.md Abschnitt 5: "60 s gültig").
const oneTimeCodeTTL = 60 * time.Second

// sessionCookieName ist der Name des Sitzungs-Cookies.
const sessionCookieName = "dmarc_session"

// auth verwaltet Instanz-Geheimnis, den aktuell ausstehenden Einmal-Code
// und die eine Sitzung dieses Serverlaufs (MIGRATIONSPLAN.md Abschnitt 5).
// Bewusst keine Mehrbenutzer-/Mehrsitzungs-Verwaltung: die Oberfläche ist
// für genau einen Nutzer auf demselben Rechner gedacht — ein einziges,
// beim Start erzeugtes Sitzungs-Token reicht.
type auth struct {
	// instanceSecret authentifiziert einen zweiten Programmstart gegenüber
	// der bereits laufenden Instanz (POST /intern/code, Meilenstein M1) —
	// hier schon erzeugt und in instance.json abgelegt, aber der Endpunkt
	// selbst kommt erst mit der Einzelinstanz-Erkennung.
	instanceSecret string
	// sessionToken gilt für die gesamte Prozesslaufzeit — kein Ablauf,
	// keine Erneuerung; das Beenden des Prozesses beendet die Sitzung.
	sessionToken string
	// csrfToken schützt zustandsändernde Anfragen zusätzlich zu
	// SameSite=Strict (siehe middleware.go requireCSRF,
	// MIGRATIONSPLAN.md Abschnitt 5). Wie sessionToken für die gesamte
	// Prozesslaufzeit gültig — ein einzelner Nutzer, eine Sitzung.
	csrfToken string

	mu          sync.Mutex
	pendingCode string
	codeExpires time.Time
}

// newAuth erzeugt Instanz-Geheimnis und Sitzungs-Token aus
// kryptografischem Zufall.
func newAuth() (*auth, error) {
	secret, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	session, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	return &auth{instanceSecret: secret, sessionToken: session, csrfToken: csrf}, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("zufallswert konnte nicht erzeugt werden: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// issueCode erzeugt einen neuen, oneTimeCodeTTL gültigen Einmal-Code.
// Ein zuvor ausgestellter, noch nicht eingelöster Code wird dabei
// verworfen — es gibt nie mehr als einen gültigen Code gleichzeitig.
func (a *auth) issueCode() (string, error) {
	code, err := randomHex(16)
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	a.pendingCode = code
	a.codeExpires = time.Now().Add(oneTimeCodeTTL)
	a.mu.Unlock()

	return code, nil
}

// redeemCode prüft code gegen den aktuell ausstehenden Einmal-Code. Der
// ausstehende Code wird in jedem Fall verworfen — bei Erfolg wie bei
// Fehlschlag —, damit ein Code nie zweimal verwendet werden kann (auch
// nicht durch einen zweiten, mitgehörten Versuch).
func (a *auth) redeemCode(code string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	valid := code != "" &&
		a.pendingCode != "" &&
		subtle.ConstantTimeCompare([]byte(code), []byte(a.pendingCode)) == 1 &&
		time.Now().Before(a.codeExpires)

	a.pendingCode = ""
	a.codeExpires = time.Time{}
	return valid
}

// validSession meldet, ob r ein gültiges Sitzungs-Cookie mitbringt.
func (a *auth) validSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(a.sessionToken)) == 1
}

// validInstanceSecret meldet, ob secret mit dem Instanz-Geheimnis dieses
// Serverlaufs übereinstimmt — Grundlage für POST /intern/code
// (Einzelinstanz-Erkennung, MIGRATIONSPLAN.md Abschnitt 3/7).
func (a *auth) validInstanceSecret(secret string) bool {
	return secret != "" && subtle.ConstantTimeCompare([]byte(secret), []byte(a.instanceSecret)) == 1
}

// validCSRFToken meldet, ob token mit dem CSRF-Token dieser Sitzung
// übereinstimmt (siehe middleware.go requireCSRF).
func (a *auth) validCSRFToken(token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.csrfToken)) == 1
}

// setSessionCookie setzt das Sitzungs-Cookie nach erfolgreichem Einlösen
// eines Einmal-Codes. HttpOnly + SameSite=Strict (MIGRATIONSPLAN.md
// Abschnitt 5); kein Secure-Flag — der Server spricht bewusst nur
// Klartext-HTTP auf 127.0.0.1, kein TLS geplant (siehe Migrationsplan,
// offene Frage 4 für den Fall, dass sich das später ändert).
func (a *auth) setSessionCookie(w http.ResponseWriter) {
	//nolint:gosec // G124: kein Secure-Flag ist hier bewusst — reines
	// Klartext-HTTP auf 127.0.0.1, kein TLS geplant (siehe Kommentar
	// oben und MIGRATIONSPLAN.md offene Frage 4).
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    a.sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
