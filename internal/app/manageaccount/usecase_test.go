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
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{accounts: make(map[account.AccountID]account.MailAccount)}
}

func (f *fakeAccountRepository) Save(_ context.Context, a *account.MailAccount) error {
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

func testAccount(t *testing.T, id account.AccountID) *account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(id, "Test", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return acc
}

func TestTestConnectionByID_Success_ClosesConnection(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	require.NoError(t, accounts.Save(context.Background(), testAccount(t, "acc-1")))
	source := &fakeMessageSource{}
	uc := &manageaccount.UseCase{
		Accounts:  accounts,
		Secret:    account.NewSecretFromString("x"),
		NewSource: func() sync.MessageSource { return source },
	}

	err := uc.TestConnectionByID(context.Background(), "acc-1")
	require.NoError(t, err)
	require.True(t, source.closed, "Verbindungstest muss die Verbindung wieder schließen")
}

func TestTestConnectionByID_ConnectFails_ReturnsError(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	require.NoError(t, accounts.Save(context.Background(), testAccount(t, "acc-1")))
	source := &fakeMessageSource{connectErr: errTest}
	uc := &manageaccount.UseCase{
		Accounts:  accounts,
		NewSource: func() sync.MessageSource { return source },
	}

	err := uc.TestConnectionByID(context.Background(), "acc-1")
	require.Error(t, err)
}

func TestTestConnectionByID_UnknownAccount_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := &manageaccount.UseCase{
		Accounts:  newFakeAccountRepository(),
		NewSource: func() sync.MessageSource { return &fakeMessageSource{} },
	}

	err := uc.TestConnectionByID(context.Background(), "unbekannt")
	require.Error(t, err)
}

func TestList_ReturnsAllAccounts(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	require.NoError(t, accounts.Save(context.Background(), testAccount(t, "acc-1")))
	require.NoError(t, accounts.Save(context.Background(), testAccount(t, "acc-2")))
	uc := &manageaccount.UseCase{Accounts: accounts}

	all, err := uc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 2)
}
