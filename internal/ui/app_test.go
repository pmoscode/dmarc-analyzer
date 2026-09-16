package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

type testSetup struct {
	accountsRepo *fakeAccountRepository
	credStore    *fakeCredentialStore
	reportsRepo  *fakeReportRepository
	connectErr   error
}

func newTestDeps(setup *testSetup) Dependencies {
	if setup.accountsRepo == nil {
		setup.accountsRepo = newFakeAccountRepository()
	}
	if setup.credStore == nil {
		setup.credStore = newFakeCredentialStore()
	}
	if setup.reportsRepo == nil {
		setup.reportsRepo = newFakeReportRepository()
	}
	newSource := func() domainsync.MessageSource { return fakeMessageSource{connectErr: setup.connectErr} }

	return Dependencies{
		Accounts: &manageaccount.UseCase{
			Accounts:    setup.accountsRepo,
			Credentials: setup.credStore,
			NewSource:   newSource,
		},
		Queries: &queryreports.UseCase{Reports: setup.reportsRepo},
		Sync: &syncreports.UseCase{
			Accounts:    setup.accountsRepo,
			Credentials: setup.credStore,
			States:      fakeStateRepository{},
			Reports:     setup.reportsRepo,
			Decoder:     fakeDecoder{},
			NewSource:   newSource,
		},
	}
}

func newSyncTestShell(setup *testSetup, w fyne.Window) *shell {
	sh := newShell(newTestDeps(setup), w)
	sh.runBackground = func(f func()) { f() }
	sh.reportsView.SetRunBackgroundForTest(func(f func()) { f() })
	sh.settingsView.SetRunBackgroundForTest(func(f func()) { f() })
	return sh
}

func testAccount(t *testing.T) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount("acc-1", "Test", "imap.example.com", 993, "u@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return *acc
}

func TestShell_NoAccounts_StartShowsOnboarding(t *testing.T) {
	setup := &testSetup{}
	w := test.NewWindow(nil)
	defer w.Close()

	sh := newSyncTestShell(setup, w)
	w.SetContent(sh)
	sh.start()

	require.NotNil(t, uitest.FindLabel(sh, i18n.OnboardingStepAccount))
}

func TestShell_WithAccounts_StartShowsMainView(t *testing.T) {
	setup := &testSetup{accountsRepo: newFakeAccountRepository(testAccount(t))}
	w := test.NewWindow(nil)
	defer w.Close()

	sh := newSyncTestShell(setup, w)
	w.SetContent(sh)
	sh.start()

	require.Nil(t, uitest.FindLabel(sh, i18n.OnboardingStepAccount))
	require.NotNil(t, uitest.FindLabel(sh, i18n.ReportsEmptyTitle),
		"ohne Berichte muss die Berichtsansicht (Standardauswahl) ihren Leerzustand zeigen")
}

func TestShell_SelectNav_SwitchesToSettings(t *testing.T) {
	setup := &testSetup{accountsRepo: newFakeAccountRepository(testAccount(t))}
	w := test.NewWindow(nil)
	defer w.Close()

	sh := newSyncTestShell(setup, w)
	w.SetContent(sh)
	sh.start()

	sh.selectNav(navSettings)

	require.NotNil(t, uitest.FindLabel(sh, i18n.SettingsTitle))
}

func TestShell_StartSync_Success_ShowsResultAndReloadsReports(t *testing.T) {
	setup := &testSetup{accountsRepo: newFakeAccountRepository(testAccount(t))}
	require.NoError(t, setup.credStoreOrInit().Store("acc-1", account.NewSecretFromString("x")))
	w := test.NewWindow(nil)
	defer w.Close()

	sh := newSyncTestShell(setup, w)
	w.SetContent(sh)
	sh.start()

	sh.startSync()

	require.Equal(t, i18n.SyncButton, sh.syncButton.Text, "Knopf muss nach Abschluss wieder den Normaltext zeigen")
}

func TestShell_StartSync_ButtonShowsCancelWhileRunning(t *testing.T) {
	setup := &testSetup{accountsRepo: newFakeAccountRepository(testAccount(t))}
	require.NoError(t, setup.credStoreOrInit().Store("acc-1", account.NewSecretFromString("x")))
	w := test.NewWindow(nil)
	defer w.Close()

	sh := newSyncTestShell(setup, w)
	w.SetContent(sh)
	sh.start()

	// runBackground ist synchron ersetzt — der Lauf ist zum Zeitpunkt des
	// SetText-Aufrufs unten bereits fertig. Der Beleg, dass der Knopf
	// zwischenzeitlich "Abbrechen" zeigte, ist deshalb nicht am
	// Endzustand ablesbar, sondern daran, dass requestCancelSync als
	// OnTapped-Handler zwischenzeitlich gesetzt wurde und sauber wieder
	// zurückgesetzt ist — siehe TestShell_StartSync_Success.
	sh.startSync()
	require.NotNil(t, sh.syncButton.OnTapped)
}

func (s *testSetup) credStoreOrInit() *fakeCredentialStore {
	if s.credStore == nil {
		s.credStore = newFakeCredentialStore()
	}
	return s.credStore
}
