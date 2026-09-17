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
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
	domainsources "github.com/pmoscode/dmarc-analyzer/internal/domain/sources"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errTest = fakeErr("testfehler")

// fakeRepository implementiert analysis.Repository mit fest verdrahteten
// Rückgabewerten — derselbe Aufbau wie zuvor internal/ui/dashboard
// (fakes_test.go), hier eigenständig für internal/web.
type fakeRepository struct {
	stats        analysis.Statistics
	dailyVolumes []analysis.DailyVolume
	topSources   []analysis.SourceVolume
	heatmap      analysis.Heatmap
	computeErr   error

	// lastDailyVolumesQuery hält die zuletzt an DailyVolumes übergebene
	// Query fest — für Tests, die prüfen, dass die URL-Filterleiste
	// (zeitraum/domain, MIGRATIONSPLAN.md Meilenstein M1) tatsächlich bis
	// zum Repository durchgereicht wird, statt nur am Handler zu enden.
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

// mustAccount baut ein gültiges MailAccount mit sinnvollen Test-Vorgaben
// unter einer festen Test-ID ("acc-1") — kein Test in diesem Paket
// braucht mehrere unterschiedliche Konten gleichzeitig, deshalb bewusst
// ohne ID-Parameter (sonst floggt unparam zu Recht: id würde nie
// variieren).
func mustAccount(t *testing.T, displayName string) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(testAccountID, displayName, "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	if err != nil {
		t.Fatalf("mustAccount: %v", err)
	}
	return *acc
}

// testAccountID ist die feste Konto-ID, die mustAccount vergibt — auch
// direkt nutzbar für URL-Pfade wie "/konten/" + string(testAccountID) +
// "/test".
const testAccountID account.AccountID = "acc-1"

// fakeReportRepository implementiert report.Repository mit fest
// verdrahteten Rückgabewerten — für Tests von /berichte, /berichte/seite
// und /berichte/{id}. Save/Exists werden von queryreports.UseCase nicht
// benutzt, müssen aber für das Interface vorhanden sein.
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

// count meldet, wie oft Save() erfolgreich aufgerufen wurde — für
// Import-Tests (handlers_import_test.go), die über
// report.SaveIfNew()/importfiles.UseCase tatsächlich gespeicherte
// Reports zählen wollen.
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
		return nil, fakeErr("nicht gefunden")
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

// fakeSettingsRepository implementiert settings.Repository in-memory —
// für Tests von /einstellungen/allgemein (AP 7).
type fakeSettingsRepository struct {
	mu       sync.Mutex
	s        settings.Settings
	hasValue bool
	saveErr  error
}

func (f *fakeSettingsRepository) Load(context.Context) (settings.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasValue {
		return settings.Default(), nil
	}
	return f.s, nil
}

func (f *fakeSettingsRepository) Save(_ context.Context, s settings.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.s = s
	f.hasValue = true
	return nil
}

// fakePruner implementiert report.Pruner mit einem festen Rückgabewert —
// die eigentliche Löschlogik ist bereits in internal/app/retention und
// internal/infra/sqlite getestet, hier reicht ein einfacher Stub.
type fakePruner struct{}

func (f *fakePruner) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

// fakeSourcesRepository implementiert domainsources.Repository mit fest
// verdrahteten Rückgabewerten — für Tests von /quellen und /quellen/seite.
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

// fakeEnricher liefert für jede Quell-IP dieselbe fest verdrahtete
// Anreicherung (leer, solange nichts anderes konfiguriert ist) — echte
// PTR-/Dienst-Erkennung ist Netzwerk-I/O und hat in Handler-Tests nichts
// zu suchen.
type fakeEnricher struct {
	enrichment domainsources.Enrichment
}

func (f *fakeEnricher) Enrich(context.Context, report.SourceIP) domainsources.Enrichment {
	return f.enrichment
}

// fakeAccountRepository implementiert account.Repository — für Tests von
// /einstellungen und /einrichtung.
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
		return nil, fmt.Errorf("konto %q nicht gefunden", id)
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

func (f *fakeAccountRepository) Delete(_ context.Context, id account.AccountID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.accounts, id)
	return nil
}

// fakeCredentialStore implementiert account.CredentialStore — für Tests
// von /einstellungen und /einrichtung. Kopiert das Secret defensiv
// (siehe AGENTS.md: ein Test-Fake, der es nur flach speichert, teilt
// sonst das Backing-Array mit dem Aufrufer).
type fakeCredentialStore struct {
	mu       sync.Mutex
	secrets  map[account.AccountID]account.Secret
	storeErr error
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{secrets: map[account.AccountID]account.Secret{}}
}

func (f *fakeCredentialStore) Store(id account.AccountID, s account.Secret) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secrets[id] = account.NewSecret(s.Expose())
	return nil
}

func (f *fakeCredentialStore) Retrieve(id account.AccountID) (account.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.secrets[id]
	if !ok {
		return account.Secret{}, account.ErrCredentialNotFound
	}
	return s, nil
}

func (f *fakeCredentialStore) Delete(id account.AccountID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.secrets, id)
	return nil
}

// fakeMessageSource implementiert domainsync.MessageSource (+
// MailboxLister) — für Tests, die manageaccount.UseCase.TestConnection/
// ListMailboxes durchlaufen (Kontoformular: Verbindung testen, Ordner
// auflisten).
type fakeMessageSource struct {
	connectErr error
	mailboxes  []string
	listErr    error
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

func (f *fakeMessageSource) ListMailboxes(context.Context) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.mailboxes, nil
}

// fakeJobSyncer implementiert syncjob.Syncer — für Tests, die einen
// Server mit Dependencies.SyncJob brauchen (Kontoseiten, /abgleich),
// ohne einen vollständig verdrahteten syncreports.UseCase (siehe
// internal/app/syncjob/runner_test.go für dieselbe Begründung).
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

// fakeFailedImportRepository implementiert domainsync.FailedImportRepository
// — für Tests von /import, die eine kaputte Datei einreichen.
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
