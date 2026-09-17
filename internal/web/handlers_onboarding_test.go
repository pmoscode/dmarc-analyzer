package web

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHandleOnboarding_NoAccounts_ShowsAccountStep(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einrichtung")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `name="host"`)
}

func TestHandleOnboarding_AccountsExist_RedirectsToDashboard(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einrichtung")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Übersicht", "muss auf die Übersicht umgeleitet haben")
}

func TestHandleOnboarding_StepTestWithoutPending_RedirectsToStepOne(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/einrichtung?schritt=test")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `name="host"`, "ohne vorherigen Schritt 1 muss wieder Schritt 1 gezeigt werden")
}

func TestOnboardingFlow_AccountStep_InvalidData_ShowsError(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt":  "konto",
		"port":     "993",
		"passwort": "geheim",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Host darf nicht leer sein")
}

func TestOnboardingFlow_CompleteHappyPath(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	// Schritt 1: Konto eingeben.
	resp := postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt":      "konto",
		"anzeigename":  "Mein Konto",
		"host":         "imap.example.com",
		"port":         "993",
		"benutzername": "user@example.com",
		"postfach":     "INBOX",
		"tls":          "on",
		"passwort":     "geheim123",
	}))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "Verbindung testen", "Schritt 2 muss angezeigt werden")

	// Konto darf vor bestandenem Verbindungstest noch nicht gespeichert sein.
	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, accounts)

	// Schritt 2: Verbindung testen — bei Erfolg wird jetzt gespeichert.
	resp = postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{"schritt": "test"}))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "Jetzt abholen", "Schritt 3 muss angezeigt werden")

	accounts, err = fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, "Mein Konto", accounts[0].DisplayName)

	secret, err := fd.credentials.Retrieve(accounts[0].ID)
	require.NoError(t, err)
	require.Equal(t, "geheim123", string(secret.Expose()))

	// Schritt 3: Ersten Abgleich anstoßen.
	resp = postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt": "abgleich",
		"aktion":  "jetzt",
	}))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "Übersicht", "muss nach Abschluss auf die Übersicht führen")

	require.Eventually(t, func() bool { return fd.syncer.callCount() > 0 }, time.Second, 5*time.Millisecond,
		"der erste Abgleich muss tatsächlich gestartet worden sein")
}

func TestOnboardingFlow_TestStepFails_AccountNotCreated(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	fd.source.connectErr = errTest
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt":      "konto",
		"host":         "imap.example.com",
		"port":         "993",
		"benutzername": "user@example.com",
		"passwort":     "geheim123",
	}))
	require.NoError(t, resp.Body.Close())

	resp = postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{"schritt": "test"}))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "Verbindung fehlgeschlagen")

	accounts, err := fd.accounts.FindAll(t.Context())
	require.NoError(t, err)
	require.Empty(t, accounts, "ein fehlgeschlagener Verbindungstest darf das Konto nicht speichern")
}

func TestOnboardingFlow_SkipFirstSync_DoesNotStartSync(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t)
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt":      "konto",
		"host":         "imap.example.com",
		"port":         "993",
		"benutzername": "user@example.com",
		"passwort":     "geheim123",
	}))
	require.NoError(t, resp.Body.Close())

	resp = postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{"schritt": "test"}))
	require.NoError(t, resp.Body.Close())

	resp = postForm(t, client, srv.Addr(), "/einrichtung", csrfFormValues(srv, map[string]string{
		"schritt": "abgleich",
		"aktion":  "ueberspringen",
	}))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, 0, fd.syncer.callCount())
}
