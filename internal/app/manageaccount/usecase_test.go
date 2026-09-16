package manageaccount_test

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var errTest = errors.New("testfehler")

type fakeAccountRepository struct {
	accounts map[account.AccountID]account.MailAccount
	saveErr  error
	delErr   error
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{accounts: make(map[account.AccountID]account.MailAccount)}
}

func (f *fakeAccountRepository) Save(_ context.Context, a *account.MailAccount) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.accounts[a.ID] = *a
	return nil
}

func (f *fakeAccountRepository) FindByID(_ context.Context, id account.AccountID) (*account.MailAccount, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, errTest
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
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.accounts, id)
	return nil
}

type fakeCredentialStore struct {
	secrets  map[account.AccountID]account.Secret
	storeErr error
	delErr   error
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{secrets: make(map[account.AccountID]account.Secret)}
}

// Store kopiert secret defensiv, siehe Kommentar in den anderen Fakes
// dieses Musters (z. B. internal/app/syncreports/fakes_test.go).
func (f *fakeCredentialStore) Store(id account.AccountID, s account.Secret) error {
	if f.storeErr != nil {
		return f.storeErr
	}
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
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.secrets, id)
	return nil
}

type fakeMessageSource struct {
	connectErr error
	closed     bool
}

func (f *fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return f.connectErr
}

func (f *fakeMessageSource) FetchNew(_ context.Context, s sync.State) (iter.Seq2[sync.RawMessage, error], sync.State, error) {
	return func(func(sync.RawMessage, error) bool) {}, s, nil
}

func (f *fakeMessageSource) Close() error {
	f.closed = true
	return nil
}

// fakeMailboxListingSource ergänzt fakeMessageSource um
// sync.MailboxLister — für Tests von ListMailboxes, ohne dass jede
// fakeMessageSource diesen (optionalen) Port implementieren müsste.
type fakeMailboxListingSource struct {
	fakeMessageSource
	mailboxes []string
	listErr   error
}

func (f *fakeMailboxListingSource) ListMailboxes(context.Context) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.mailboxes, nil
}

func testAccount(t *testing.T, id account.AccountID) *account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(id, "Test", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return acc
}

func TestCreate_SavesMetadataAndCredentials(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	uc := &manageaccount.UseCase{Accounts: accounts, Credentials: creds}

	acc := testAccount(t, "acc-1")
	secret := account.NewSecretFromString("app-passwort")
	require.NoError(t, uc.Create(context.Background(), acc, secret))

	require.Contains(t, accounts.accounts, account.AccountID("acc-1"))
	got, err := creds.Retrieve("acc-1")
	require.NoError(t, err)
	require.Equal(t, "app-passwort", string(got.Expose()))
}

func TestCreate_CredentialStoreFails_RollsBackMetadata(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	creds.storeErr = errTest
	uc := &manageaccount.UseCase{Accounts: accounts, Credentials: creds}

	err := uc.Create(context.Background(), testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.Error(t, err)
	require.NotContains(t, accounts.accounts, account.AccountID("acc-1"),
		"ein Konto ohne Zugangsdaten darf nicht übrig bleiben")
}

func TestDelete_RemovesMetadataAndCredentials(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	uc := &manageaccount.UseCase{Accounts: accounts, Credentials: creds}

	require.NoError(t, uc.Create(context.Background(), testAccount(t, "acc-1"), account.NewSecretFromString("x")))
	require.NoError(t, uc.Delete(context.Background(), "acc-1"))

	require.NotContains(t, accounts.accounts, account.AccountID("acc-1"))
	_, err := creds.Retrieve("acc-1")
	require.ErrorIs(t, err, account.ErrCredentialNotFound)
}

func TestDelete_AttemptsBothHalvesEvenIfOneFails(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	creds := newFakeCredentialStore()
	uc := &manageaccount.UseCase{Accounts: accounts, Credentials: creds}
	require.NoError(t, uc.Create(context.Background(), testAccount(t, "acc-1"), account.NewSecretFromString("x")))

	creds.delErr = errTest
	err := uc.Delete(context.Background(), "acc-1")
	require.Error(t, err, "Fehler bei einer Hälfte muss gemeldet werden")

	// Trotz Fehler beim Schlüsselbund: die Metadaten-Hälfte wurde
	// trotzdem versucht und ist tatsächlich weg.
	require.NotContains(t, accounts.accounts, account.AccountID("acc-1"))
}

func TestTestConnection_Success(t *testing.T) {
	t.Parallel()

	source := &fakeMessageSource{}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	err := uc.TestConnection(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.NoError(t, err)
	require.True(t, source.closed, "Verbindungstest muss die Verbindung wieder schließen")
}

func TestTestConnection_Failure(t *testing.T) {
	t.Parallel()

	source := &fakeMessageSource{connectErr: errTest}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	err := uc.TestConnection(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.Error(t, err)
}

func TestListMailboxes_Success_ReturnsMailboxesAndClosesConnection(t *testing.T) {
	t.Parallel()

	source := &fakeMailboxListingSource{mailboxes: []string{"INBOX", "INBOX/DMARC"}}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	got, err := uc.ListMailboxes(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.NoError(t, err)
	require.Equal(t, []string{"INBOX", "INBOX/DMARC"}, got)
	require.True(t, source.closed, "Postfachliste muss die Verbindung wieder schließen")
}

func TestListMailboxes_ConnectFails_ReturnsError(t *testing.T) {
	t.Parallel()

	source := &fakeMailboxListingSource{fakeMessageSource: fakeMessageSource{connectErr: errTest}}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	_, err := uc.ListMailboxes(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.Error(t, err)
}

func TestListMailboxes_ListFails_ReturnsError(t *testing.T) {
	t.Parallel()

	source := &fakeMailboxListingSource{listErr: errTest}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	_, err := uc.ListMailboxes(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.ErrorIs(t, err, errTest)
}

func TestListMailboxes_SourceWithoutMailboxLister_ReturnsClearError(t *testing.T) {
	t.Parallel()

	source := &fakeMessageSource{}
	uc := &manageaccount.UseCase{NewSource: func() sync.MessageSource { return source }}

	_, err := uc.ListMailboxes(context.Background(), *testAccount(t, "acc-1"), account.NewSecretFromString("x"))
	require.Error(t, err)
}

func TestList_ReturnsAllAccounts(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	uc := &manageaccount.UseCase{Accounts: accounts, Credentials: newFakeCredentialStore()}

	require.NoError(t, uc.Create(context.Background(), testAccount(t, "acc-1"), account.NewSecretFromString("x")))
	require.NoError(t, uc.Create(context.Background(), testAccount(t, "acc-2"), account.NewSecretFromString("y")))

	all, err := uc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 2)
}
