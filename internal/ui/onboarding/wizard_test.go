package onboarding

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
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

func newTestWizard(t *testing.T, setup *testSetup, w fyne.Window) *Wizard {
	t.Helper()
	if setup == nil {
		setup = &testSetup{}
	}
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

	accounts := &manageaccount.UseCase{
		Accounts:    setup.accountsRepo,
		Credentials: setup.credStore,
		NewSource:   newSource,
	}
	sync := &syncreports.UseCase{
		Accounts:    setup.accountsRepo,
		Credentials: setup.credStore,
		States:      fakeStateRepository{},
		Reports:     setup.reportsRepo,
		Decoder:     fakeDecoder{},
		NewSource:   newSource,
	}

	wiz := NewWizard(accounts, sync, w)
	wiz.runBackground = func(f func()) { f() }
	return wiz
}

// fillValidAccountForm trägt gültige Werte ein. wiz.form ist vom Typ
// *settings.AccountForm (fremdes Paket, Felder dort unexported) — die
// Entry-Widgets werden deshalb über uitest.FindEntries angesprochen,
// in der Reihenfolge, in der settings.NewAccountForm sie anlegt:
// Anzeigename, Host, Port, Benutzername, Postfach, Passwort (Check
// "Verschlüsselte Verbindung" ist kein Entry und taucht hier nicht auf).
func fillValidAccountForm(wiz *Wizard) {
	entries := uitest.FindEntries(wiz.form)
	const (
		idxHost     = 1
		idxUsername = 3
		idxPassword = 5
	)
	entries[idxHost].SetText("imap.example.com")
	entries[idxUsername].SetText("user@example.com")
	entries[idxPassword].SetText("app-passwort")
}

func TestWizard_InitialStep_ShowsAccountForm(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, nil, w)
	w.SetContent(wiz)

	require.Equal(t, stepAccount, wiz.current)
	require.NotNil(t, uitest.FindLabel(wiz, i18n.OnboardingStepAccount))
}

func TestWizard_SubmitAccountStep_InvalidForm_ShowsErrorAndStaysOnStep(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, nil, w)
	w.SetContent(wiz)

	wiz.submitAccountStep() // Formular ist leer

	require.Equal(t, stepAccount, wiz.current)
	require.True(t, wiz.banner.Visible())
}

func TestWizard_SubmitAccountStep_ValidForm_AdvancesToTestStep(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, nil, w)
	w.SetContent(wiz)
	fillValidAccountForm(wiz)

	wiz.submitAccountStep()

	require.Equal(t, stepTest, wiz.current)
	require.NotNil(t, uitest.FindLabel(wiz, i18n.OnboardingStepTest))
}

func TestWizard_ConnectionTest_Success_CreatesAccountAndAdvances(t *testing.T) {
	setup := &testSetup{}
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, setup, w)
	w.SetContent(wiz)
	fillValidAccountForm(wiz)
	wiz.submitAccountStep()

	wiz.runConnectionTest()

	require.Equal(t, stepFirstSync, wiz.current)
	require.Len(t, setup.accountsRepo.accounts, 1, "Konto muss nach erfolgreichem Test gespeichert sein")
	require.NotNil(t, uitest.FindLabel(wiz, i18n.OnboardingStepFirstSync))
}

func TestWizard_ConnectionTest_Failure_ShowsErrorAndStaysOnStep(t *testing.T) {
	setup := &testSetup{connectErr: errNotFound}
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, setup, w)
	w.SetContent(wiz)
	fillValidAccountForm(wiz)
	wiz.submitAccountStep()

	wiz.runConnectionTest()

	require.Equal(t, stepTest, wiz.current)
	require.True(t, wiz.banner.Visible())
	require.Empty(t, setup.accountsRepo.accounts, "bei fehlgeschlagenem Test darf nichts gespeichert werden")
}

func TestWizard_FirstSyncStep_Skip_CallsOnComplete(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, nil, w)
	w.SetContent(wiz)
	fillValidAccountForm(wiz)
	wiz.submitAccountStep()
	wiz.runConnectionTest()
	require.Equal(t, stepFirstSync, wiz.current)

	completed := false
	wiz.OnComplete = func() { completed = true }

	skipButton := uitest.FindButton(t, wiz, i18n.OnboardingFirstSyncSkip)
	test.Tap(skipButton)

	require.True(t, completed)
}

func TestWizard_FirstSyncStep_RunSync_Success_ShowsResultThenCompletes(t *testing.T) {
	w := test.NewWindow(nil)
	defer w.Close()

	wiz := newTestWizard(t, nil, w)
	w.SetContent(wiz)
	fillValidAccountForm(wiz)
	wiz.submitAccountStep()
	wiz.runConnectionTest()
	require.Equal(t, stepFirstSync, wiz.current)

	wiz.runFirstSync()

	// Nach erfolgreichem Sync zeigt der Assistent zunächst das Ergebnis,
	// bevor er sich mit OnComplete schließt — kein sofortiges
	// Verschwinden ohne Rückmeldung.
	require.NotNil(t, uitest.FindLabel(wiz, i18n.OnboardingDone))

	completed := false
	wiz.OnComplete = func() { completed = true }
	okButton := uitest.FindButton(t, wiz, i18n.ButtonOK)
	test.Tap(okButton)

	require.True(t, completed)
}
