package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// defaultIMAPPort/mailboxProbeAccountID: dieselben Vorgaben wie zuvor
// internal/ui/settings.AccountForm (defaultIMAPPort, mailboxProbeAccount)
// — Port 993, ein fester Platzhalter-Konto-ID für den probeweisen
// Verbindungsaufbau beim Auflisten der Postfächer (noch kein echtes
// Konto, wird nirgends gespeichert).
const (
	defaultIMAPPort       = 993
	mailboxProbeAccountID = account.AccountID("mailbox-lookup-probe")
	defaultMailboxForForm = "INBOX"
)

// accountFormView sind die Werte des Kontoformulars, so wie sie erneut
// angezeigt werden sollen — nie inklusive Passwort (siehe
// parseAccountFormView-Dokumentation).
type accountFormView struct {
	DisplayName    string
	Host           string
	Port           string
	Username       string
	Mailbox        string
	MailboxOptions []string
	UseTLS         bool
}

func defaultAccountFormView() accountFormView {
	return accountFormView{Port: strconv.Itoa(defaultIMAPPort), Mailbox: defaultMailboxForForm, UseTLS: true}
}

// parseAccountFormView liest die nicht-geheimen Formularfelder — das
// Passwort wird bewusst NIE in eine accountFormView übernommen: Ein
// erneut angezeigtes Formular (nach einem Validierungsfehler oder nach
// "Ordner auflisten") lässt das Passwortfeld immer leer, der Nutzer muss
// es erneut eingeben. Dieselbe, verbreitete Konvention wie bei den
// meisten Web-Formularen — ein Passwort im value-Attribut wäre über
// "Seitenquelltext anzeigen" lesbar, obwohl es serverseitig längst nicht
// mehr gebraucht wird.
func parseAccountFormView(r *http.Request) accountFormView {
	tls := r.PostFormValue("tls") != ""
	return accountFormView{
		DisplayName: strings.TrimSpace(r.PostFormValue("anzeigename")),
		Host:        strings.TrimSpace(r.PostFormValue("host")),
		Port:        strings.TrimSpace(r.PostFormValue("port")),
		Username:    strings.TrimSpace(r.PostFormValue("benutzername")),
		Mailbox:     strings.TrimSpace(r.PostFormValue("postfach")),
		UseTLS:      tls,
	}
}

// buildAccountFromForm validiert die Formularwerte und baut daraus ein
// MailAccount plus das eingegebene Secret — dieselben Regeln wie zuvor
// internal/ui/settings.AccountForm.Validate()/BuildAccount().
func buildAccountFromForm(r *http.Request, id account.AccountID) (*account.MailAccount, account.Secret, error) {
	view := parseAccountFormView(r)
	password := r.PostFormValue("passwort")

	if view.Host == "" {
		return nil, account.Secret{}, fmt.Errorf("host darf nicht leer sein")
	}
	if view.Username == "" {
		return nil, account.Secret{}, fmt.Errorf("benutzername darf nicht leer sein")
	}
	port, err := strconv.Atoi(view.Port)
	if err != nil || port < 1 || port > 65535 {
		return nil, account.Secret{}, fmt.Errorf("port ist ungültig")
	}
	if password == "" {
		return nil, account.Secret{}, fmt.Errorf("passwort darf nicht leer sein")
	}

	acc, err := account.NewMailAccount(id, view.DisplayName, view.Host, port, view.Username, view.Mailbox, view.UseTLS, time.Now())
	if err != nil {
		return nil, account.Secret{}, err
	}
	return acc, account.NewSecretFromString(password), nil
}

type accountRowView struct {
	ID          string
	DisplayName string
	Host        string
	Port        int
	Username    string
	Mailbox     string
}

func accountRows(accounts []account.MailAccount) []accountRowView {
	rows := make([]accountRowView, len(accounts))
	for i, a := range accounts {
		rows[i] = accountRowView{
			ID:          string(a.ID),
			DisplayName: a.DisplayName,
			Host:        a.Host,
			Port:        a.Port,
			Username:    a.Username,
			Mailbox:     a.Mailbox,
		}
	}
	return rows
}

type settingsPageData struct {
	Title string
	Nav   []navItem

	Accounts []accountRowView
	Form     accountFormView

	Message        string
	MessageIsError bool
}

func (s *Server) renderSettingsWithMessage(w http.ResponseWriter, r *http.Request, message string, isError bool, form accountFormView) {
	accounts, err := s.deps.Accounts.List(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := settingsPageData{
		Title:          "Einstellungen",
		Nav:            navItems("/einstellungen"),
		Accounts:       accountRows(accounts),
		Form:           form,
		Message:        message,
		MessageIsError: isError,
	}
	if err := s.views.render(w, "settings.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

func redirectToSettingsWithMessage(w http.ResponseWriter, r *http.Request, message string, isError bool) {
	v := url.Values{}
	v.Set("meldung", message)
	if isError {
		v.Set("art", "fehler")
	}
	http.Redirect(w, r, "/einstellungen?"+v.Encode(), http.StatusSeeOther)
}

// handleSettings zeigt die Kontenübersicht plus Formular für ein neues
// Konto (MIGRATIONSPLAN.md Abschnitt 7: "GET /einstellungen").
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.deps.Accounts.List(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := settingsPageData{
		Title:          "Einstellungen",
		Nav:            navItems(r.URL.Path),
		Accounts:       accountRows(accounts),
		Form:           defaultAccountFormView(),
		Message:        r.URL.Query().Get("meldung"),
		MessageIsError: r.URL.Query().Get("art") == "fehler",
	}
	if err := s.views.render(w, "settings.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleAccountCreate legt ein neues Konto an (MIGRATIONSPLAN.md
// Abschnitt 7: "POST /konten") — ohne vorherigen Verbindungstest, wie
// zuvor internal/ui/settings.View.submitNewAccount (im Unterschied zur
// Ersteinrichtung, die vor dem Speichern testet, siehe
// handlers_onboarding.go).
func (s *Server) handleAccountCreate(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einstellungen")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.serverError(w, r, err)
		return
	}

	acc, secret, err := buildAccountFromForm(r, account.AccountID(uuid.NewString()))
	if err != nil {
		s.renderSettingsWithMessage(w, r, displayError(err), true, parseAccountFormView(r))
		return
	}

	createErr := s.deps.Accounts.Create(r.Context(), acc, secret)
	secret.Zero()
	if createErr != nil {
		s.renderSettingsWithMessage(w, r, "Konto konnte nicht gespeichert werden.", true, parseAccountFormView(r))
		return
	}

	redirectToSettingsWithMessage(w, r, "Konto gespeichert.", false)
}

// handleAccountListMailboxes probt die eingegebenen (noch nicht
// gespeicherten) Verbindungsdaten und listet die vorhandenen Postfächer
// auf — Grundlage des Ordner-Pickers (MIGRATIONSPLAN.md Abschnitt 7:
// "POST /konten/ordner"). Rendert dieselbe Einstellungen-Seite erneut,
// mit den erkannten Postfächern in der Auswahlliste, statt per htmx nur
// einen Ausschnitt auszutauschen — vermeidet, das Passwort für einen
// zusätzlichen Ajax-Aufruf zwischenzuspeichern (siehe
// parseAccountFormView-Dokumentation).
func (s *Server) handleAccountListMailboxes(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einstellungen")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.serverError(w, r, err)
		return
	}

	acc, secret, err := buildAccountFromForm(r, mailboxProbeAccountID)
	if err != nil {
		s.renderSettingsWithMessage(w, r, displayError(err), true, parseAccountFormView(r))
		return
	}
	defer secret.Zero()

	mailboxes, err := s.deps.Accounts.ListMailboxes(r.Context(), *acc, secret)
	form := parseAccountFormView(r)
	if err != nil {
		s.renderSettingsWithMessage(w, r, "Postfächer konnten nicht abgerufen werden — Verbindungsdaten prüfen.", true, form)
		return
	}

	form.MailboxOptions = mailboxes
	s.renderSettingsWithMessage(w, r, fmt.Sprintf("%d Postfächer gefunden.", len(mailboxes)), false, form)
}

// handleAccountTest testet die Verbindung eines bereits gespeicherten
// Kontos (MIGRATIONSPLAN.md Abschnitt 7: "POST /konten/{id}/test").
func (s *Server) handleAccountTest(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einstellungen")
		return
	}
	id := account.AccountID(r.PathValue("id"))

	if err := s.deps.Accounts.TestConnectionByID(r.Context(), id); err != nil {
		redirectToSettingsWithMessage(w, r, "Verbindung fehlgeschlagen.", true)
		return
	}
	redirectToSettingsWithMessage(w, r, "Verbindung erfolgreich.", false)
}

// handleAccountDelete löscht ein Konto vollständig — Metadaten und
// Zugangsdaten (MIGRATIONSPLAN.md Abschnitt 7: "POST
// /konten/{id}/loeschen").
func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	if s.credentialsLocked() {
		s.redirectToUnlock(w, r, "/einstellungen")
		return
	}
	id := account.AccountID(r.PathValue("id"))

	if err := s.deps.Accounts.Delete(r.Context(), id); err != nil {
		redirectToSettingsWithMessage(w, r, "Konto konnte nicht gelöscht werden.", true)
		return
	}
	redirectToSettingsWithMessage(w, r, "Konto gelöscht.", false)
}
