package web

import (
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// onboardingState hält das in Schritt 1 des Ersteinrichtungs-Assistenten
// gebaute, noch nicht gespeicherte Konto (samt Secret) zwischen den
// HTTP-Anfragen der einzelnen Schritte fest — bewusst serverseitig im
// Speicher statt über versteckte Formularfelder durch den Browser
// gereicht, damit das Passwort nicht als Klartext im HTML-Quelltext der
// Zwischenschritte auftaucht (siehe handlers_settings.go,
// parseAccountFormView-Dokumentation, dieselbe Begründung). Passt zum
// Ein-Sitzung-Modell der ganzen Oberfläche (MIGRATIONSPLAN.md Abschnitt
// 5: "genau einen Nutzer auf demselben Rechner") — ein einziger,
// gemeinsamer Zustand statt einer pro Sitzung verwalteten Map.
type onboardingState struct {
	mu      sync.Mutex
	account *account.MailAccount
	secret  account.Secret
}

func (o *onboardingState) set(acc *account.MailAccount, secret account.Secret) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.account = acc
	o.secret = secret
}

func (o *onboardingState) get() (*account.MailAccount, account.Secret, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.account == nil {
		return nil, account.Secret{}, false
	}
	return o.account, o.secret, true
}

// clear leert das gemerkte Secret aktiv (statt es dem Garbage Collector
// zu überlassen) und vergisst das Konto — nach erfolgreichem Speichern
// (Schritt 2) oder Abbruch des Assistenten.
func (o *onboardingState) clear() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.secret.Zero()
	o.account = nil
	o.secret = account.Secret{}
}

type onboardingPageData struct {
	Title string
	Nav   []navItem

	Step  string // "konto" | "test" | "abgleich"
	Form  accountFormView
	Error string
}

// handleOnboarding zeigt den jeweils aktuellen Schritt der
// Ersteinrichtung (MIGRATIONSPLAN.md Abschnitt 7: "GET/POST
// /einrichtung"). Schritt 2/3 ohne vorher abgeschlossenen Schritt 1
// (z. B. Lesezeichen, Server neu gestartet): zurück auf Schritt 1 statt
// mit nil-Konto weiterzumachen.
func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einrichtung")
		return
	}

	step := r.URL.Query().Get("schritt")
	_, _, hasPending := s.onboarding.get()

	if (step == "test" || step == "abgleich") && !hasPending {
		http.Redirect(w, r, "/einrichtung", http.StatusSeeOther)
		return
	}

	switch step {
	case "test":
		s.renderOnboarding(w, r, "test", accountFormView{}, "")
	case "abgleich":
		s.renderOnboarding(w, r, "abgleich", accountFormView{}, "")
	default:
		// Direkter Einstieg (kein Formularfehler): schon Konten
		// vorhanden UND kein Assistent gerade in Bearbeitung → die
		// Ersteinrichtung ist nicht (mehr) gemeint, zurück zur
		// Übersicht (die umgekehrte Weiterleitung, siehe
		// handlers_dashboard.go).
		if !hasPending && s.deps.Accounts != nil {
			if accounts, err := s.deps.Accounts.List(r.Context()); err == nil && len(accounts) > 0 {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		s.renderOnboarding(w, r, "konto", defaultAccountFormView(), "")
	}
}

func (s *Server) renderOnboarding(w http.ResponseWriter, r *http.Request, step string, form accountFormView, errMsg string) {
	data := onboardingPageData{
		Title: "Ersteinrichtung",
		Nav:   navItems("/einrichtung"),
		Step:  step,
		Form:  form,
		Error: errMsg,
	}
	if err := s.views.render(w, "onboarding.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleOnboardingSubmit verzweigt anhand des versteckten "schritt"-Felds
// an den jeweiligen Schritt — ein Formular pro Schritt statt eines
// gemeinsamen Multi-Step-Formulars, dieselbe serverseitige Wizard-Logik
// wie zuvor internal/ui/onboarding.Wizard.
func (s *Server) handleOnboardingSubmit(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einrichtung")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.serverError(w, r, err)
		return
	}

	switch r.PostFormValue("schritt") {
	case "test":
		s.handleOnboardingTestSubmit(w, r)
	case "abgleich":
		s.handleOnboardingSyncSubmit(w, r)
	default:
		s.handleOnboardingAccountSubmit(w, r)
	}
}

// handleOnboardingAccountSubmit ist Schritt 1: Formularwerte validieren,
// noch NICHT speichern (siehe Schritt 2 unten — es wird erst nach
// erfolgreichem Verbindungstest gespeichert, wie zuvor
// internal/ui/onboarding.Wizard.submitAccountStep/runConnectionTest).
func (s *Server) handleOnboardingAccountSubmit(w http.ResponseWriter, r *http.Request) {
	acc, secret, err := buildAccountFromForm(r, account.AccountID(uuid.NewString()))
	if err != nil {
		s.renderOnboarding(w, r, "konto", parseAccountFormView(r), displayError(err))
		return
	}

	s.onboarding.set(acc, secret)
	http.Redirect(w, r, "/einrichtung?schritt=test", http.StatusSeeOther)
}

// handleOnboardingTestSubmit ist Schritt 2: Verbindung mit den in Schritt
// 1 eingegebenen (noch ungespeicherten) Daten testen; erst bei Erfolg
// wird das Konto tatsächlich gespeichert.
func (s *Server) handleOnboardingTestSubmit(w http.ResponseWriter, r *http.Request) {
	acc, secret, ok := s.onboarding.get()
	if !ok {
		http.Redirect(w, r, "/einrichtung", http.StatusSeeOther)
		return
	}

	if err := s.deps.Accounts.TestConnection(r.Context(), *acc, secret); err != nil {
		s.renderOnboarding(w, r, "test", accountFormView{}, "Verbindung fehlgeschlagen — bitte die Angaben in Schritt 1 prüfen.")
		return
	}

	if err := s.deps.Accounts.Create(r.Context(), acc, secret); err != nil {
		s.renderOnboarding(w, r, "test", accountFormView{}, "Konto konnte nicht gespeichert werden.")
		return
	}
	// Ab hier ist das Konto gespeichert — das Secret im Zwischenspeicher
	// wird nicht mehr gebraucht; account.MailAccount bleibt (für den
	// Anzeigenamen/ID) bis zum Ende von Schritt 3 erhalten.
	secret.Zero()

	http.Redirect(w, r, "/einrichtung?schritt=abgleich", http.StatusSeeOther)
}

// handleOnboardingSyncSubmit ist Schritt 3: ersten Abgleich anstoßen oder
// überspringen. Der Abgleich selbst läuft asynchron über den
// serverweiten syncjob.Runner — sein Fortschritt ist ab jetzt auf jeder
// Seite sichtbar (Sync-Anzeige in layout.html), ein eigener
// Zwischenschritt "Ergebnis anzeigen" (wie zuvor
// internal/ui/onboarding.Wizard.showDoneStep) entfällt deshalb bewusst.
func (s *Server) handleOnboardingSyncSubmit(w http.ResponseWriter, r *http.Request) {
	if r.PostFormValue("aktion") == "jetzt" {
		_ = s.deps.SyncJob.Start() // ErrAlreadyRunning ist hier kein Fehlerfall
	}
	s.onboarding.clear()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
