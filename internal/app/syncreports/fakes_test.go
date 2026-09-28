package syncreports_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	stdsync "sync"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// --- account.Repository -----------------------------------------------

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
		return nil, fmt.Errorf("account %q: %w", id, errNotFound)
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

var errNotFound = errors.New("not found")

// --- domainsync.StateRepository -----------------------------------------

type fakeStateRepository struct {
	mu     stdsync.Mutex
	states map[string]domainsync.State
	// saves counts every Save call — tests use this to verify that
	// progress is actually persisted multiple times, not just once at
	// the end.
	saves []domainsync.State
}

func newFakeStateRepository() *fakeStateRepository {
	return &fakeStateRepository{states: make(map[string]domainsync.State)}
}

func stateKey(id account.AccountID, mailbox string) string {
	return string(id) + "\x00" + mailbox
}

func (f *fakeStateRepository) Load(_ context.Context, id account.AccountID, mailbox string) (domainsync.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.states[stateKey(id, mailbox)]; ok {
		return s, nil
	}
	return domainsync.State{AccountID: id, Mailbox: mailbox}, nil
}

func (f *fakeStateRepository) Save(_ context.Context, s domainsync.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[stateKey(s.AccountID, s.Mailbox)] = s
	f.saves = append(f.saves, s)
	return nil
}

// --- report.Repository ---------------------------------------------------

type fakeReportRepository struct {
	mu        stdsync.Mutex
	byKey     map[report.Key]*report.AggregateReport
	nextID    int64
	saveErr   error
	existsErr error
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{byKey: make(map[report.Key]*report.AggregateReport)}
}

func (f *fakeReportRepository) Save(_ context.Context, r *report.AggregateReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.saveErr != nil {
		return f.saveErr
	}
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
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.byKey[key]
	return ok, nil
}

func (f *fakeReportRepository) FindByID(_ context.Context, id report.ReportID) (*report.AggregateReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.byKey {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("report %d: %w", id, errNotFound)
}

func (f *fakeReportRepository) Query(context.Context, report.Query) (report.Page, error) {
	return report.Page{}, nil
}

func (f *fakeReportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byKey)
}

// --- domainsync.FailedImportRepository ------------------------------------

type fakeFailedImportRepository struct {
	mu      stdsync.Mutex
	entries []domainsync.FailedImport
}

func newFakeFailedImportRepository() *fakeFailedImportRepository {
	return &fakeFailedImportRepository{}
}

func (f *fakeFailedImportRepository) Record(_ context.Context, entry domainsync.FailedImport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeFailedImportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}

// --- domainsync.MessageDecoder ---------------------------------------------

// fakeDecoder treats every message as exactly one attachment with the
// message content as filename — enough for tests that don't actually
// want to parse MIME (internal/infra/mailmime already tests that
// separately).
type fakeDecoder struct{}

func (fakeDecoder) Decode(data []byte) ([]domainsync.RawAttachment, error) {
	return []domainsync.RawAttachment{{Filename: string(data), Data: data}}, nil
}

// failingDecoder returns the same error for every message — simulates a
// broken MIME structure.
type failingDecoder struct{ err error }

func (d failingDecoder) Decode([]byte) ([]domainsync.RawAttachment, error) {
	return nil, d.err
}

// --- domainsync.ReportParser -----------------------------------------------

// fakeParser accepts every attachment and returns a report whose
// ReportID matches the attachment content — or a preprogrammed error if
// the attachment content is in failFor.
type fakeParser struct {
	failFor map[string]error
}

func (p fakeParser) Supports(domainsync.RawAttachment) bool { return true }

func (p fakeParser) Parse(_ context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	content := string(att.Data)
	if err, ok := p.failFor[content]; ok {
		return nil, err
	}

	domain, err := report.NewDomainName("example.com")
	if err != nil {
		return nil, err
	}
	dateRange, err := report.NewDateRange(fixedBegin, fixedBegin.Add(hour))
	if err != nil {
		return nil, err
	}
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	if err != nil {
		return nil, err
	}

	return report.NewAggregateReport(
		report.Metadata{OrgName: "fake-org.example", ReportID: content, Range: dateRange},
		policy, nil, report.SourceReference{}, fixedBegin,
	)
}

// fakeMultiParser additionally implements domainsync.MultiReportParser
// — simulates an attachment (e.g. a .zip) that contains multiple reports
// at once. Attachment content "multi:a,b,c" yields three reports with
// ReportIDs a, b, c.
type fakeMultiParser struct{}

func (p fakeMultiParser) Supports(domainsync.RawAttachment) bool { return true }

func (p fakeMultiParser) Parse(ctx context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	all, err := p.ParseAll(ctx, att)
	if err != nil {
		return nil, err
	}
	return all[0], nil
}

func (p fakeMultiParser) ParseAll(_ context.Context, att domainsync.RawAttachment) ([]*report.AggregateReport, error) {
	ids := strings.Split(strings.TrimPrefix(string(att.Data), "multi:"), ",")

	domain, err := report.NewDomainName("example.com")
	if err != nil {
		return nil, err
	}
	dateRange, err := report.NewDateRange(fixedBegin, fixedBegin.Add(hour))
	if err != nil {
		return nil, err
	}
	policy, err := report.NewPublishedPolicy(domain, report.PolicyReject, report.PolicyReject, report.AlignmentRelaxed, report.AlignmentRelaxed, 100, "")
	if err != nil {
		return nil, err
	}

	reports := make([]*report.AggregateReport, 0, len(ids))
	for _, id := range ids {
		rep, err := report.NewAggregateReport(
			report.Metadata{OrgName: "fake-org.example", ReportID: id, Range: dateRange},
			policy, nil, report.SourceReference{}, fixedBegin,
		)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, nil
}

// --- domainsync.MessageSource -----------------------------------------------

// fakeMessageSource returns a preprogrammed list of messages.
// connectErr/fetchErr simulate connection or fetch errors respectively.
type fakeMessageSource struct {
	messages   []domainsync.RawMessage
	connectErr error
	fetchErr   error
	// iterErr, if set, is delivered as the last (msg, err) pair after all
	// messages — simulates an error in the middle of the iterator (e.g.
	// context cancellation, see internal/infra/imap/fetch.go).
	iterErr error

	closed bool
}

func (f *fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return f.connectErr
}

func (f *fakeMessageSource) FetchNew(ctx context.Context, state domainsync.State) (iter.Seq2[domainsync.RawMessage, error], domainsync.State, error) {
	if f.fetchErr != nil {
		return nil, state, f.fetchErr
	}

	baseline := state
	seq := func(yield func(domainsync.RawMessage, error) bool) {
		for _, msg := range f.messages {
			select {
			case <-ctx.Done():
				yield(domainsync.RawMessage{}, ctx.Err())
				return
			default:
			}
			if !yield(msg, nil) {
				return
			}
		}
		if f.iterErr != nil {
			yield(domainsync.RawMessage{}, f.iterErr)
		}
	}
	return seq, baseline, nil
}

func (f *fakeMessageSource) Close() error {
	f.closed = true
	return nil
}
