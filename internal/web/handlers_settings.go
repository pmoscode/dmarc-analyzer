package web

import (
	"net/http"
	"net/url"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// accountRowView ist die für die Status-Seite aufbereitete Sicht auf das
// eine, per ENV konfigurierte Konto (nie das Passwort — das steht nur im
// Prozessspeicher, siehe internal/infra/envconfig).
type accountRowView struct {
	ID          string
	DisplayName string
	Host        string
	Port        int
	Username    string
	Mailbox     string
	UseTLS      bool
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
			UseTLS:      a.UseTLS,
		}
	}
	return rows
}

// settingsPageData sind die Werte für die reine Status-Seite (AP 7 /
// Docker-Umstieg): Konto und Laufzeit-Einstellungen kommen komplett aus
// ENV (internal/infra/envconfig) und werden hier nur angezeigt, nicht
// mehr bearbeitet — Ändern braucht einen Container-Neustart mit anderen
// Umgebungsvariablen.
type settingsPageData struct {
	Title string
	Nav   []navItem

	Accounts            []accountRowView
	RetentionMonths     int
	SyncIntervalMinutes int

	Message        string
	MessageIsError bool
}

func redirectToSettingsWithMessage(w http.ResponseWriter, r *http.Request, message string, isError bool) {
	v := url.Values{}
	v.Set("meldung", message)
	if isError {
		v.Set("art", "fehler")
	}
	http.Redirect(w, r, "/einstellungen?"+v.Encode(), http.StatusSeeOther)
}

// handleSettings zeigt das per ENV konfigurierte Konto sowie die aktuell
// geltende Aufbewahrungsdauer/den Sync-Intervall an.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.deps.Accounts.List(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	data := settingsPageData{
		Title:               "Status",
		Nav:                 navItems(r.URL.Path),
		Accounts:            accountRows(accounts),
		RetentionMonths:     s.deps.Retention.RetentionMonths,
		SyncIntervalMinutes: s.deps.SyncIntervalMinutes,
		Message:             r.URL.Query().Get("meldung"),
		MessageIsError:      r.URL.Query().Get("art") == "fehler",
	}
	if err := s.views.render(w, r, "settings.html", data); err != nil {
		s.serverError(w, r, err)
	}
}

// handleAccountTest testet die Verbindung des konfigurierten Kontos.
func (s *Server) handleAccountTest(w http.ResponseWriter, r *http.Request) {
	id := account.AccountID(r.PathValue("id"))

	if err := s.deps.Accounts.TestConnectionByID(r.Context(), id); err != nil {
		redirectToSettingsWithMessage(w, r, "Verbindung fehlgeschlagen.", true)
		return
	}
	redirectToSettingsWithMessage(w, r, "Verbindung erfolgreich.", false)
}
