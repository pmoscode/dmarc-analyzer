package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
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

// generalFormView sind die Werte des Formulars für programmweite
// Einstellungen (AP 7), als Strings statt int: ein Validierungsfehler
// (z. B. Buchstaben im Zahlenfeld) muss die eingegebene Zeichenkette
// unverändert zurückgeben können, dieselbe Konvention wie accountFormView.
type generalFormView struct {
	RetentionMonths     string
	SyncIntervalMinutes string
}

func generalFormFromSettings(s settings.Settings) generalFormView {
	return generalFormView{
		RetentionMonths:     strconv.Itoa(s.RetentionMonths),
		SyncIntervalMinutes: strconv.Itoa(s.SyncIntervalMinutes),
	}
}

func parseGeneralFormView(r *http.Request) generalFormView {
	return generalFormView{
		RetentionMonths:     strings.TrimSpace(r.PostFormValue("aufbewahrung_monate")),
		SyncIntervalMinutes: strings.TrimSpace(r.PostFormValue("sync_intervall_minuten")),
	}
}

// buildSettingsFromForm validiert die Formularwerte — negative Zahlen
// lehnt bereits settings.Settings.Validate() ab, hier zusätzlich die reine
// Zahlenform, die strconv.Atoi allein nicht verständlich meldet.
func buildSettingsFromForm(view generalFormView) (settings.Settings, error) {
	retention, err := strconv.Atoi(view.RetentionMonths)
	if err != nil || retention < 0 {
		return settings.Settings{}, fmt.Errorf("aufbewahrungsdauer muss eine nicht-negative ganze zahl sein")
	}
	interval, err := strconv.Atoi(view.SyncIntervalMinutes)
	if err != nil || interval < 0 {
		return settings.Settings{}, fmt.Errorf("sync-intervall muss eine nicht-negative ganze zahl sein")
	}
	return settings.Settings{RetentionMonths: retention, SyncIntervalMinutes: interval}, nil
}

type settingsPageData struct {
	Title string
	Nav   []navItem

	Accounts []accountRowView
	Form     accountFormView

	General generalFormView

	Message        string
	MessageIsError bool
}

// loadGeneralForm liest die aktuell gespeicherten Einstellungen für die
// Anzeige — bei einem Fehler (z. B. beschädigte Einstellungsdatei) zeigt
// die Seite die Vorgabewerte statt ganz zu scheitern: die Kontenverwaltung
// auf derselben Seite bleibt davon unabhängig nutzbar.
func (s *Server) loadGeneralForm(ctx context.Context) generalFormView {
	current, err := s.deps.Retention.LoadSettings(ctx)
	if err != nil {
		current = settings.Default()
	}
	return generalFormFromSettings(current)
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
		General:        s.loadGeneralForm(r.Context()),
		Message:        message,
		MessageIsError: isError,
	}
	if err := s.views.render(w, "settings.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// renderSettingsWithGeneralMessage entspricht renderSettingsWithMessage,
// zeigt aber die (möglicherweise ungültig eingegebenen) Formularwerte des
// Allgemein-Formulars erneut an, statt die zuletzt gespeicherten zu laden
// — dieselbe Konvention wie bei buildAccountFromForm/renderSettingsWithMessage
// für das Kontoformular.
func (s *Server) renderSettingsWithGeneralMessage(w http.ResponseWriter, r *http.Request, message string, isError bool, general generalFormView) {
	accounts, err := s.deps.Accounts.List(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := settingsPageData{
		Title:          "Einstellungen",
		Nav:            navItems("/einstellungen"),
		Accounts:       accountRows(accounts),
		Form:           defaultAccountFormView(),
		General:        general,
		Message:        message,
		MessageIsError: isError,
	}
	if err := s.views.render(w, "settings.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleGeneralSettingsUpdate speichert Aufbewahrungsdauer und
// Sync-Intervall (AP 7, MIGRATIONSPLAN.md-Nachfolgeabschnitt "Einstellungen
// erweitert um Aufbewahrung/Hintergrund-Sync") — braucht keinen
// entsperrten Schlüsselbund, anders als die Kontoformulare.
func (s *Server) handleGeneralSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.serverError(w, r, err)
		return
	}

	view := parseGeneralFormView(r)
	newSettings, err := buildSettingsFromForm(view)
	if err != nil {
		s.renderSettingsWithGeneralMessage(w, r, displayError(err), true, view)
		return
	}

	if err := s.deps.Retention.SaveSettings(r.Context(), newSettings); err != nil {
		s.renderSettingsWithGeneralMessage(w, r, "Einstellungen konnten nicht gespeichert werden.", true, view)
		return
	}

	redirectToSettingsWithMessage(w, r, "Einstellungen gespeichert.", false)
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
		General:        s.loadGeneralForm(r.Context()),
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
