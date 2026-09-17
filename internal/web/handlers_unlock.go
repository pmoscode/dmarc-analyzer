package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// lockChecker wird optional von einem account.CredentialStore erfüllt
// (aktuell: keyring.LockableFileStore) — der OS-Schlüsselbund-Adapter
// kennt keinen Sperrzustand (Locked() liefert er nie, weil er die
// Schnittstelle gar nicht implementiert).
type lockChecker interface{ Locked() bool }

// unlocker wird optional von einem account.CredentialStore erfüllt.
type unlocker interface {
	Unlock(secret account.Secret) error
}

// locker wird optional von einem account.CredentialStore erfüllt — für
// den Fall, dass sich eine per Unlock() angenommene Passphrase
// nachträglich als falsch herausstellt (siehe handleUnlockSubmit).
type locker interface{ Lock() }

// credentialsLocked meldet, ob der aktuelle CredentialStore gesperrt ist
// (MIGRATIONSPLAN.md Erweiterung 9.4). false für alles, was keinen
// Sperrzustand kennt (OS-Schlüsselbund, nil in Tests ohne
// Kontenverwaltung).
func (s *Server) credentialsLocked() bool {
	if s.deps.Credentials == nil {
		return false
	}
	lc, ok := s.deps.Credentials.(lockChecker)
	return ok && lc.Locked()
}

// redirectToUnlock leitet auf /entsperren um und merkt sich path als
// Rücksprungziel nach erfolgreichem Entsperren.
func (s *Server) redirectToUnlock(w http.ResponseWriter, r *http.Request, path string) {
	v := url.Values{}
	v.Set("weiter", path)
	http.Redirect(w, r, "/entsperren?"+v.Encode(), http.StatusSeeOther)
}

// nextOrDefault lässt nur einen eigenen, absoluten Pfad ("/…") als
// Rücksprungziel zu — nie ein Schema-Host-Ziel, das ein manipulierter
// "next"-Parameter sonst als Open-Redirect missbrauchen könnte.
func nextOrDefault(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/"
}

type unlockPageData struct {
	Title string
	Nav   []navItem
	Error string
	Next  string
}

// handleUnlockForm zeigt die Entsperr-Seite (MIGRATIONSPLAN.md
// Erweiterung 9.4/Abschnitt 7: "GET/POST /entsperren"). Bereits
// entsperrt (oder OS-Schlüsselbund, der nie sperrt): direkt weiter zum
// gemerkten Ziel, kein sinnloser Zwischenstopp.
func (s *Server) handleUnlockForm(w http.ResponseWriter, r *http.Request) {
	if !s.credentialsLocked() {
		//nolint:gosec // G710: nextOrDefault lässt nur einen eigenen,
		// mit genau einem "/" beginnenden Pfad durch (siehe dort).
		http.Redirect(w, r, nextOrDefault(r.URL.Query().Get("weiter")), http.StatusSeeOther)
		return
	}

	data := unlockPageData{Title: "Entsperren", Nav: navItems(r.URL.Path), Next: r.URL.Query().Get("weiter")}
	if err := s.views.render(w, "unlock.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) renderUnlockError(w http.ResponseWriter, r *http.Request, next, message string) {
	data := unlockPageData{Title: "Entsperren", Nav: navItems("/entsperren"), Error: message, Next: next}
	if err := s.views.render(w, "unlock.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleUnlockSubmit verarbeitet die eingegebene Master-Passphrase.
// Unlock() selbst prüft die Passphrase nicht aktiv (siehe
// keyring.LockableFileStore-Dokumentation: ohne ein bereits gespeichertes
// Secret gibt es nichts, wogegen zu prüfen wäre) — existiert mindestens
// ein Konto, wird die Passphrase deshalb zusätzlich über einen echten
// Retrieve()-Versuch verifiziert; schlägt der fehl, wird wieder
// gesperrt und ein Fehler gezeigt, statt den Nutzer mit einer
// stillschweigend falschen Passphrase weiterzuschicken.
func (s *Server) handleUnlockSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.serverError(w, r, err)
		return
	}
	next := r.PostFormValue("next")
	passphrase := account.NewSecretFromString(r.PostFormValue("passphrase"))
	defer passphrase.Zero()

	unlockable, ok := s.deps.Credentials.(unlocker)
	if !ok {
		s.serverError(w, r, fmt.Errorf("credential-store unterstützt kein entsperren"))
		return
	}

	if err := unlockable.Unlock(passphrase); err != nil {
		s.renderUnlockError(w, r, next, "Passphrase konnte nicht verarbeitet werden.")
		return
	}

	if s.deps.Accounts != nil {
		if accounts, err := s.deps.Accounts.List(r.Context()); err == nil && len(accounts) > 0 {
			_, retrieveErr := s.deps.Credentials.Retrieve(accounts[0].ID)
			// ErrCredentialNotFound bedeutet nur "für dieses Konto noch
			// nie ein Secret gespeichert" (z. B. frisch angelegter
			// Datei-Speicher) — kein Hinweis auf eine falsche Passphrase,
			// im Unterschied zu jedem anderen Fehler (Entschlüsselung
			// scheitert an falschem Schlüssel).
			if retrieveErr != nil && !errors.Is(retrieveErr, account.ErrCredentialNotFound) {
				if l, ok := s.deps.Credentials.(locker); ok {
					l.Lock()
				}
				s.renderUnlockError(w, r, next, "Falsche Master-Passphrase.")
				return
			}
		}
	}

	//nolint:gosec // G710: nextOrDefault lässt nur einen eigenen, mit
	// genau einem "/" beginnenden Pfad durch (siehe dort).
	http.Redirect(w, r, nextOrDefault(next), http.StatusSeeOther)
}
