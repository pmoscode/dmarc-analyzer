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
		return nil, fmt.Errorf("konto %q: %w", id, errNotFound)
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

var errNotFound = errors.New("nicht gefunden")

// --- account.CredentialStore -------------------------------------------

type fakeCredentialStore struct {
	secrets map[account.AccountID]account.Secret
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{secrets: make(map[account.AccountID]account.Secret)}
}

// Store kopiert secret defensiv (account.NewSecret kopiert) — reale
// Adapter (OSStore, FileStore) verwandeln die Bytes synchron in eine
// eigene Kopie (String-Konversion bzw. Verschlüsselung); ein Aufrufer darf
// das übergebene Secret direkt danach mit Zero() überschreiben (siehe
// Vertrag an account.CredentialStore). Ohne diese Kopie würde ein
// späteres Zero() beim Aufrufer auch den hier "gespeicherten" Wert
// leeren, weil beide dasselbe Backing-Array teilen.
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

// --- domainsync.StateRepository -----------------------------------------

type fakeStateRepository struct {
	mu     stdsync.Mutex
	states map[string]domainsync.State
	// saves zählt jeden Save-Aufruf — Tests nutzen das, um zu prüfen, dass
	// der Fortschritt tatsächlich mehrfach fortgeschrieben wird, nicht nur
	// einmal am Ende.
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

// fakeDecoder behandelt jede Nachricht als genau einen Anhang mit dem
// Nachrichteninhalt als Dateiname — reicht für Tests, die nicht wirklich
// MIME parsen wollen (das prüft internal/infra/mailmime bereits separat).
type fakeDecoder struct{}

func (fakeDecoder) Decode(data []byte) ([]domainsync.RawAttachment, error) {
	return []domainsync.RawAttachment{{Filename: string(data), Data: data}}, nil
}

// failingDecoder liefert für jede Nachricht denselben Fehler — simuliert
// eine kaputte MIME-Struktur.
type failingDecoder struct{ err error }

func (d failingDecoder) Decode([]byte) ([]domainsync.RawAttachment, error) {
	return nil, d.err
}

// --- domainsync.ReportParser -----------------------------------------------

// fakeParser akzeptiert jeden Anhang und liefert einen Report, dessen
// ReportID dem Anhangsinhalt entspricht — oder einen vorprogrammierten
// Fehler, wenn der Anhangsinhalt in failFor steht.
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

// fakeMultiParser implementiert zusätzlich domainsync.MultiReportParser
// — simuliert einen Anhang (z. B. ein .zip), der mehrere Reports auf
// einmal enthält. Anhangsinhalt "multi:a,b,c" liefert drei Reports mit
// den ReportIDs a, b, c.
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

// fakeMessageSource liefert eine vorprogrammierte Liste von Nachrichten.
// connectErr/fetchErr simulieren Verbindungs- bzw. Fetch-Fehler.
type fakeMessageSource struct {
	messages   []domainsync.RawMessage
	connectErr error
	fetchErr   error
	// iterErr wird nach allen messages als letztes (msg, err)-Paar
	// geliefert, wenn gesetzt — simuliert einen Fehler mitten im Iterator
	// (z. B. Kontext-Abbruch, siehe internal/infra/imap/fetch.go).
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
