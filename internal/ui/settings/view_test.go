package settings

import (
	"context"
	"iter"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/i18n"
	"github.com/pmoscode/dmarc-analyzer/internal/ui/uitest"
)

// --- Fakes ---------------------------------------------------------------

type fakeAccountRepository struct {
	accounts map[account.AccountID]account.MailAccount
}

func newFakeAccountRepository(accs ...account.MailAccount) *fakeAccountRepository {
	m := make(map[account.AccountID]account.MailAccount, len(accs))
	for _, a := range accs {
		m[a.ID] = a
	}
	return &fakeAccountRepository{accounts: m}
}

func (f *fakeAccountRepository) Save(_ context.Context, a *account.MailAccount) error {
	f.accounts[a.ID] = *a
	return nil
}

func (f *fakeAccountRepository) FindByID(_ context.Context, id account.AccountID) (*account.MailAccount, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, errNotFound
	}
	return &a, nil
}

func (f *fakeAccountRepository) FindAll(context.Context) ([]account.MailAccount, error) {
	all := make([]account.MailAccount, 0, len(f.accounts))
	for _, a := range f.accounts {
		all = append(all, a)
	}
	return all, nil
}

func (f *fakeAccountRepository) Delete(_ context.Context, id account.AccountID) error {
	delete(f.accounts, id)
	return nil
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errNotFound = fakeErr("nicht gefunden")

type fakeCredentialStore struct {
	secrets map[account.AccountID]account.Secret
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{secrets: make(map[account.AccountID]account.Secret)}
}

func (f *fakeCredentialStore) Store(id account.AccountID, s account.Secret) error {
	f.secrets[id] = account.NewSecret(s.Expose())
	return nil
}

func (f *fakeCredentialStore) Retrieve(id account.AccountID) (account.Secret, error) {
	s, ok := f.secrets[id]
	if !ok {
		return account.Secret{}, account.ErrCredentialNotFound
	}
	return s, nil
}

func (f *fakeCredentialStore) Delete(id account.AccountID) error {
	delete(f.secrets, id)
	return nil
}

// --- Tests -----------------------------------------------------------------

func testAccount(t *testing.T) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount("acc-1", "Test-Konto", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return *acc
}

// newSyncTestView baut eine View mit synchronem runBackground — die
// Fyne-Testtreiber führt fyne.Do() ohnehin nur inline auf der
// aufrufenden Goroutine aus (keine echte Marshalling wie beim echten
// Treiber), eine echte Hintergrund-Goroutine würde Tests nur ohne jeden
// Nutzen nebenläufig zum Testcode selbst machen — siehe AGENTS.md.
func newSyncTestView(accounts *manageaccount.UseCase, window fyne.Window) *View {
	v := NewView(accounts, window)
	v.runBackground = func(f func()) { f() }
	return v
}

func TestView_NoAccounts_ShowsEmptyState(t *testing.T) {
	uc := &manageaccount.UseCase{
		Accounts:    newFakeAccountRepository(),
		Credentials: newFakeCredentialStore(),
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)
	v.Reload()

	require.NotNil(t, uitest.FindLabel(v, i18n.SettingsEmptyTitle))
}

func TestView_WithAccounts_HidesEmptyState(t *testing.T) {
	acc := testAccount(t)
	uc := &manageaccount.UseCase{
		Accounts:    newFakeAccountRepository(acc),
		Credentials: newFakeCredentialStore(),
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)
	v.Reload()

	require.Nil(t, uitest.FindLabel(v, i18n.SettingsEmptyTitle))
	require.Equal(t, 1, len(v.data))
}

func TestView_Reload_PicksUpNewAccount(t *testing.T) {
	repo := newFakeAccountRepository()
	uc := &manageaccount.UseCase{Accounts: repo, Credentials: newFakeCredentialStore()}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)
	v.Reload()
	require.NotNil(t, uitest.FindLabel(v, i18n.SettingsEmptyTitle))

	repo.accounts["acc-1"] = testAccount(t)
	v.Reload()

	require.Nil(t, uitest.FindLabel(v, i18n.SettingsEmptyTitle))
}

func TestView_SubmitNewAccount_ValidForm_CreatesAccountAndReloads(t *testing.T) {
	repo := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	uc := &manageaccount.UseCase{Accounts: repo, Credentials: creds}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)
	v.Reload()

	changed := false
	v.OnAccountsChanged = func() { changed = true }

	form := NewAccountForm()
	form.host.SetText("imap.example.com")
	form.username.SetText("user@example.com")
	form.password.SetText("app-passwort")

	v.submitNewAccount(form)

	require.Len(t, repo.accounts, 1)
	require.True(t, changed)
	require.Nil(t, uitest.FindLabel(v, i18n.SettingsEmptyTitle))
}

func TestView_SubmitNewAccount_InvalidForm_DoesNotCreateAccount(t *testing.T) {
	repo := newFakeAccountRepository()
	uc := &manageaccount.UseCase{Accounts: repo, Credentials: newFakeCredentialStore()}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)

	form := NewAccountForm() // Host/Username/Passwort leer
	v.submitNewAccount(form)

	require.Empty(t, repo.accounts)
}

// fakeMessageSource ist ein minimaler sync.MessageSource-Fake — die
// eigentliche Verbindungslogik ist in internal/app/manageaccount und
// internal/infra/imap bereits ausführlich getestet, hier geht es nur um
// die Verdrahtung: löst v.testAccount() den Use Case korrekt aus?
type fakeMessageSource struct{ connectErr error }

func (f fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return f.connectErr
}

func (fakeMessageSource) FetchNew(_ context.Context, s domainsync.State) (iter.Seq2[domainsync.RawMessage, error], domainsync.State, error) {
	return func(func(domainsync.RawMessage, error) bool) {}, s, nil
}

func (fakeMessageSource) Close() error { return nil }

func TestView_TestAccount_Success_DoesNotPanic(t *testing.T) {
	acc := testAccount(t)
	creds := newFakeCredentialStore()
	require.NoError(t, creds.Store("acc-1", account.NewSecretFromString("x")))

	uc := &manageaccount.UseCase{
		Accounts:    newFakeAccountRepository(acc),
		Credentials: creds,
		NewSource:   func() domainsync.MessageSource { return fakeMessageSource{} },
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)

	// Der eigentliche Beleg wäre ein Blick auf den gezeigten Dialog —
	// Fynes Test-Treiber bietet dafür keine stabile Introspektion.
	// Smoke-Level laut Teststrategie (IMPLEMENTIERUNG.md Abschnitt 12.2):
	// der Aufruf darf nicht abstürzen.
	require.NotPanics(t, func() { v.testAccount(acc) })
}

func TestView_TestAccount_ConnectionFails_DoesNotPanic(t *testing.T) {
	acc := testAccount(t)
	creds := newFakeCredentialStore()
	require.NoError(t, creds.Store("acc-1", account.NewSecretFromString("x")))

	uc := &manageaccount.UseCase{
		Accounts:    newFakeAccountRepository(acc),
		Credentials: creds,
		NewSource:   func() domainsync.MessageSource { return fakeMessageSource{connectErr: errNotFound} },
	}
	w := test.NewWindow(nil)
	defer w.Close()

	v := newSyncTestView(uc, w)
	w.SetContent(v)

	require.NotPanics(t, func() { v.testAccount(acc) })
}
