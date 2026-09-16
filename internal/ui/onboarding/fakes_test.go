package onboarding

import (
	"context"
	"iter"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errNotFound = fakeErr("nicht gefunden")

// --- account.Repository / account.CredentialStore -----------------------

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

// --- domainsync.MessageSource ---------------------------------------------

type fakeMessageSource struct{ connectErr error }

func (f fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return f.connectErr
}

func (fakeMessageSource) FetchNew(_ context.Context, s domainsync.State) (iter.Seq2[domainsync.RawMessage, error], domainsync.State, error) {
	return func(func(domainsync.RawMessage, error) bool) {}, s, nil
}

func (fakeMessageSource) Close() error { return nil }

// --- report.Repository (für syncreports.UseCase) --------------------------

type fakeReportRepository struct {
	byKey  map[report.Key]*report.AggregateReport
	nextID int64
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{byKey: make(map[report.Key]*report.AggregateReport)}
}

func (f *fakeReportRepository) Save(_ context.Context, r *report.AggregateReport) error {
	if _, exists := f.byKey[r.Key()]; exists {
		return report.ErrDuplicate
	}
	f.nextID++
	r.ID = report.ReportID(f.nextID)
	cp := *r
	f.byKey[r.Key()] = &cp
	return nil
}

func (f *fakeReportRepository) Exists(_ context.Context, key report.Key) (bool, error) {
	_, ok := f.byKey[key]
	return ok, nil
}

func (f *fakeReportRepository) FindByID(context.Context, report.ReportID) (*report.AggregateReport, error) {
	return nil, nil //nolint:nilnil // im Test nicht benötigt
}

func (f *fakeReportRepository) Query(context.Context, report.Query) (report.Page, error) {
	return report.Page{}, nil
}

// --- domainsync.StateRepository ---------------------------------------------

type fakeStateRepository struct{}

func (fakeStateRepository) Load(_ context.Context, id account.AccountID, mailbox string) (domainsync.State, error) {
	return domainsync.State{AccountID: id, Mailbox: mailbox}, nil
}
func (fakeStateRepository) Save(context.Context, domainsync.State) error { return nil }

// --- domainsync.MessageDecoder / ReportParser ---------------------------

type fakeDecoder struct{}

func (fakeDecoder) Decode(data []byte) ([]domainsync.RawAttachment, error) {
	return []domainsync.RawAttachment{{Filename: string(data), Data: data}}, nil
}
