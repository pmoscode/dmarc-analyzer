package web

import (
	"context"
	"fmt"
	"iter"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/domainstats"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/failedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errTest = fakeErr("test error")

// fakeRepository implements analysis.Repository with hard-wired return
// values — the same structure as formerly internal/ui/dashboard
// (fakes_test.go), here standalone for internal/web.
type fakeRepository struct {
	stats        analysis.Statistics
	dailyVolumes []analysis.DailyVolume
	topSources   []analysis.SourceVolume
	heatmap      analysis.Heatmap
	computeErr   error

	// lastDailyVolumesQuery keeps the query last passed to DailyVolumes —
	// for tests that check the URL filter bar (zeitraum/domain,
	// MIGRATIONSPLAN.md milestone M1) actually makes it all the way to
	// the repository, not just ending at the handler.
	lastDailyVolumesQuery analysis.Query
}

func (f *fakeRepository) Compute(context.Context, analysis.Query) (analysis.Statistics, error) {
	if f.computeErr != nil {
		return analysis.Statistics{}, f.computeErr
	}
	return f.stats, nil
}

func (f *fakeRepository) DailyVolumes(_ context.Context, q analysis.Query) ([]analysis.DailyVolume, error) {
	f.lastDailyVolumesQuery = q
	return f.dailyVolumes, nil
}

func (f *fakeRepository) TopSources(context.Context, analysis.Query, int) ([]analysis.SourceVolume, error) {
	return f.topSources, nil
}

func (f *fakeRepository) Heatmap(context.Context, analysis.Query, int) (analysis.Heatmap, error) {
	return f.heatmap, nil
}

func mustSourceIP(s string) report.SourceIP {
	ip, err := report.NewSourceIP(s)
	if err != nil {
		panic(err)
	}
	return ip
}

// mustAccount builds a valid MailAccount with sensible test defaults
// under a fixed test ID ("acc-1") — no test in this package needs
// several different accounts at once, hence deliberately no ID parameter
// (otherwise unparam would rightly flag it: id would never vary).
func mustAccount(t *testing.T, displayName string) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(testAccountID, displayName, "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	if err != nil {
		t.Fatalf("mustAccount: %v", err)
	}
	return *acc
}

// testAccountID is the fixed account ID mustAccount assigns — also
// directly usable for URL paths like "/konten/" + string(testAccountID)
// + "/test".
const testAccountID account.AccountID = "acc-1"

// fakeReportRepository implements report.Repository with hard-wired
// return values — for tests of /berichte, /berichte/seite and
// /berichte/{id}. Save/Exists aren't used by queryreports.UseCase, but
// must exist for the interface.
type fakeReportRepository struct {
	mu        sync.Mutex
	page      report.Page
	queryErr  error
	byID      map[report.ReportID]*report.AggregateReport
	getErr    error
	lastQuery report.Query
	saved     int
}

func (f *fakeReportRepository) Save(context.Context, *report.AggregateReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved++
	return nil
}

func (f *fakeReportRepository) Exists(context.Context, report.Key) (bool, error) {
	return false, nil
}

// count reports how many times Save() was called successfully — for
// import tests (handlers_import_test.go) that want to count reports
// actually saved via report.SaveIfNew()/importfiles.UseCase.
func (f *fakeReportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saved
}

func (f *fakeReportRepository) FindByID(_ context.Context, id report.ReportID) (*report.AggregateReport, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	full, ok := f.byID[id]
	if !ok {
		return nil, fakeErr("not found")
	}
	return full, nil
}

func (f *fakeReportRepository) Query(_ context.Context, q report.Query) (report.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return report.Page{}, f.queryErr
	}
	return f.page, nil
}

// fakePruner implements report.Pruner with a fixed return value — the
// actual deletion logic is already tested in internal/app/retention and
// internal/infra/sqlite, a simple stub suffices here.
type fakePruner struct{}

func (f *fakePruner) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

// fakeSourcesRepository implements domainsources.Repository with
// hard-wired return values — for tests of /quellen and /quellen/seite.
type fakeSourcesRepository struct {
	page     domainsources.Page
	queryErr error

	lastQuery domainsources.Query
}

func (f *fakeSourcesRepository) Query(_ context.Context, q domainsources.Query) (domainsources.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return domainsources.Page{}, f.queryErr
	}
	return f.page, nil
}

// fakeDomainsRepository implements domainstats.Repository with
// hard-wired return values — for tests of /domains and /domains/seite.
type fakeDomainsRepository struct {
	page     domainstats.Page
	queryErr error

	lastQuery domainstats.Query
}

func (f *fakeDomainsRepository) Query(_ context.Context, q domainstats.Query) (domainstats.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return domainstats.Page{}, f.queryErr
	}
	return f.page, nil
}

// fakeFailedRecordsRepository implements failedrecords.Repository with
// hard-wired return values — for tests of /fehlschlaege and
// /fehlschlaege/seite.
type fakeFailedRecordsRepository struct {
	page     failedrecords.Page
	queryErr error

	lastQuery failedrecords.Query
}

func (f *fakeFailedRecordsRepository) Query(_ context.Context, q failedrecords.Query) (failedrecords.Page, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return failedrecords.Page{}, f.queryErr
	}
	return f.page, nil
}

func mustDomainName(s string) report.DomainName {
	d, err := report.NewDomainName(s)
	if err != nil {
		panic(err)
	}
	return d
}

// fakeEnricher returns the same hard-wired enrichment for every source
// IP (empty as long as nothing else is configured) — real PTR/service
// detection is network I/O and has no place in handler tests.
type fakeEnricher struct {
	enrichment domainsources.Enrichment
}

func (f *fakeEnricher) Enrich(context.Context, report.SourceIP) domainsources.Enrichment {
	return f.enrichment
}

// fakeAccountRepository implements account.Repository — for tests of
// /einstellungen.
type fakeAccountRepository struct {
	mu         sync.Mutex
	accounts   map[account.AccountID]account.MailAccount
	findAllErr error
}

func newFakeAccountRepository(accs ...account.MailAccount) *fakeAccountRepository {
	m := make(map[account.AccountID]account.MailAccount, len(accs))
	for _, a := range accs {
		m[a.ID] = a
	}
	return &fakeAccountRepository{accounts: m}
}

func (f *fakeAccountRepository) Save(_ context.Context, a *account.MailAccount) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accounts[a.ID] = *a
	return nil
}

func (f *fakeAccountRepository) FindByID(_ context.Context, id account.AccountID) (*account.MailAccount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %q not found", id)
	}
	return &a, nil
}

func (f *fakeAccountRepository) FindAll(context.Context) ([]account.MailAccount, error) {
	if f.findAllErr != nil {
		return nil, f.findAllErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]account.MailAccount, 0, len(f.accounts))
	for _, a := range f.accounts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// fakeMessageSource implements domainsync.MessageSource — for tests that
// run through manageaccount.UseCase.TestConnectionByID ("test
// connection" on the status page).
type fakeMessageSource struct {
	connectErr error
	closed     bool
}

func (f *fakeMessageSource) Connect(context.Context, account.MailAccount, account.Secret) error {
	return f.connectErr
}

func (f *fakeMessageSource) FetchNew(context.Context, domainsync.State) (iter.Seq2[domainsync.RawMessage, error], domainsync.State, error) {
	return func(func(domainsync.RawMessage, error) bool) {}, domainsync.State{}, nil
}

func (f *fakeMessageSource) Close() error {
	f.closed = true
	return nil
}

// fakeJobSyncer implements syncjob.Syncer — for tests that need a server
// with Dependencies.SyncJob (account pages, /abgleich) without a fully
// wired-up syncreports.UseCase (see internal/app/syncjob/runner_test.go
// for the same rationale).
type fakeJobSyncer struct {
	mu     sync.Mutex
	calls  []account.AccountID
	result syncreports.Result
	err    error
	block  bool
}

func (f *fakeJobSyncer) SyncAccount(ctx context.Context, id account.AccountID, onProgress syncreports.OnProgress) (syncreports.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	f.mu.Unlock()
	if onProgress != nil {
		onProgress(syncreports.Progress{})
	}
	if f.block {
		<-ctx.Done()
		return syncreports.Result{}, ctx.Err()
	}
	return f.result, f.err
}

func (f *fakeJobSyncer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeFailedImportRepository implements
// domainsync.FailedImportRepository — for tests of /import that submit a
// broken file.
type fakeFailedImportRepository struct {
	mu      sync.Mutex
	records []domainsync.FailedImport
}

func (f *fakeFailedImportRepository) Record(_ context.Context, rec domainsync.FailedImport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, rec)
	return nil
}

func (f *fakeFailedImportRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}
