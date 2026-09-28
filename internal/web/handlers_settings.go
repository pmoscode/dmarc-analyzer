package web

import (
	"net/http"
	"net/url"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// accountRowView is the status-page-ready view of the one account
// configured via ENV (never the password — that only lives in process
// memory, see internal/infra/envconfig).
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

// settingsPageData holds the values for the plain status page (AP 7 /
// Docker migration): account and runtime settings come entirely from ENV
// (internal/infra/envconfig) and are only displayed here, no longer
// editable — changing them needs a container restart with different
// environment variables.
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

// handleSettings shows the account configured via ENV as well as the
// currently effective retention period/sync interval.
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

// handleAccountTest tests the connection of the configured account.
func (s *Server) handleAccountTest(w http.ResponseWriter, r *http.Request) {
	id := account.AccountID(r.PathValue("id"))

	if err := s.deps.Accounts.TestConnectionByID(r.Context(), id); err != nil {
		redirectToSettingsWithMessage(w, r, "Connection failed.", true)
		return
	}
	redirectToSettingsWithMessage(w, r, "Connection successful.", false)
}
