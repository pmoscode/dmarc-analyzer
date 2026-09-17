package main

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// fakeStateRepository und fakeMessageSource sind minimal, nur für den
// CLI-Dispatch-Test nötig — die eigentliche Sync-Logik ist bereits in
// internal/app/syncreports ausführlich getestet.
type fakeStateRepository struct{}

func (fakeStateRepository) Load(_ context.Context, id account.AccountID, mailbox string) (domainsync.State, error) {
	return domainsync.State{AccountID: id, Mailbox: mailbox}, nil
}
func (fakeStateRepository) Save(context.Context, domainsync.State) error { return nil }

type fakeMessageSource struct{}

func (fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return nil
}
func (fakeMessageSource) FetchNew(_ context.Context, s domainsync.State) (iter.Seq2[domainsync.RawMessage, error], domainsync.State, error) {
	return func(func(domainsync.RawMessage, error) bool) {}, s, nil
}
func (fakeMessageSource) Close() error { return nil }

func TestRunSync_NoAccounts_ReturnsError(t *testing.T) {
	t.Parallel()

	accounts := newFakeAccountRepository()
	a := &app{accounts: &manageaccount.UseCase{Accounts: accounts}}

	err := runSync(context.Background(), a, nil)
	require.Error(t, err)
}

func TestRunSync_SyncsAllConfiguredAccounts(t *testing.T) {
	t.Parallel()

	acc, err := account.NewMailAccount("acc-1", "Test", "imap.example.com", 993, "u@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)

	accountsRepo := newFakeAccountRepository(*acc)
	secret := account.NewSecretFromString("x")

	a := &app{
		accounts: &manageaccount.UseCase{Accounts: accountsRepo, Secret: secret},
		sync: &syncreports.UseCase{
			Accounts:  accountsRepo,
			Secret:    secret,
			States:    fakeStateRepository{},
			Reports:   newFakeReportRepository(),
			Decoder:   fakeDecoder{},
			Parsers:   []domainsync.ReportParser{fakeParser{}},
			NewSource: func() domainsync.MessageSource { return fakeMessageSource{} },
		},
	}

	require.NoError(t, runSync(context.Background(), a, nil))
}
