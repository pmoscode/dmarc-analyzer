package syncjob_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// fakeAccountRepository implementiert nur, was Runner braucht (FindAll)
// — Save/FindByID/Delete werden nie aufgerufen, sind aber Teil des
// account.Repository-Interfaces.
type fakeAccountRepository struct {
	accounts []account.MailAccount
	err      error
}

func (f *fakeAccountRepository) Save(context.Context, *account.MailAccount) error { return nil }
func (f *fakeAccountRepository) FindByID(context.Context, account.AccountID) (*account.MailAccount, error) {
	return nil, nil
}
func (f *fakeAccountRepository) Delete(context.Context, account.AccountID) error { return nil }
func (f *fakeAccountRepository) FindAll(context.Context) ([]account.MailAccount, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.accounts, nil
}

// fakeSyncer implementiert syncjob.Syncer mit einstellbarem Verhalten je
// Konto — kein vollständig verdrahteter syncreports.UseCase nötig
// (bräuchte IMAP-/Decoder-/Parser-Fakes, die für syncjob irrelevant
// sind: Runner kennt nur die Syncer-Schnittstelle).
type fakeSyncer struct {
	mu    sync.Mutex
	calls []account.AccountID

	result syncreports.Result
	err    error
	// block lässt SyncAccount blockieren, bis ctx abgebrochen wird — für
	// Cancel()-Tests.
	block bool
}

func (f *fakeSyncer) SyncAccount(ctx context.Context, id account.AccountID, onProgress syncreports.OnProgress) (syncreports.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	f.mu.Unlock()

	if f.block {
		<-ctx.Done()
		return syncreports.Result{}, ctx.Err()
	}

	return f.result, f.err
}

func (f *fakeSyncer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// controlledProgressSyncer meldet genau einen Fortschritts-Zwischenstand
// und blockiert danach, bis der Test proceed schließt — macht
// "kommt ein Zwischenstand tatsächlich beim Zuhörer an" deterministisch
// testbar statt von der Präferenz eines gepufferten Kanals abzuhängen
// (Runner.Subscribe liefert bewusst nur den JEWEILS NEUESTEN Stand, ein
// schneller Produzent kann Zwischenstände sonst überschreiben, bevor ein
// Testleser sie sieht).
type controlledProgressSyncer struct {
	proceed chan struct{}
}

func (f *controlledProgressSyncer) SyncAccount(ctx context.Context, _ account.AccountID, onProgress syncreports.OnProgress) (syncreports.Result, error) {
	if onProgress != nil {
		onProgress(syncreports.Progress{Processed: 1, New: 1})
	}
	select {
	case <-f.proceed:
	case <-ctx.Done():
		return syncreports.Result{}, ctx.Err()
	}
	return syncreports.Result{New: 1}, nil
}

func mustAccount(t *testing.T, id account.AccountID, displayName string) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(id, displayName, "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return *acc
}

func waitForStatus(t *testing.T, r *syncjob.Runner, want syncjob.Status) syncjob.State {
	t.Helper()
	ch, unsubscribe := r.Subscribe()
	defer unsubscribe()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case s := <-ch:
			if s.Status == want {
				return s
			}
		case <-deadline:
			t.Fatalf("status %q nicht innerhalb der Frist erreicht (zuletzt: %q)", want, r.Snapshot().Status)
		}
	}
}

func TestRunner_Start_SyncsAllAccountsSequentially(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{
		mustAccount(t, "acc-1", "Konto 1"),
		mustAccount(t, "acc-2", "Konto 2"),
	}}
	syncer := &fakeSyncer{result: syncreports.Result{New: 2, Skipped: 1}}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount())
	require.Equal(t, 4, final.Total.New) // 2 Konten × 2
	require.Equal(t, 2, final.Total.Skipped)
	require.Equal(t, 2, final.TotalAccounts)
	require.NoError(t, final.Err)
	require.False(t, final.EndedAt.IsZero())
}

func TestRunner_Start_AlreadyRunning_ReturnsError(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Konto 1")}}
	syncer := &fakeSyncer{block: true}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	err := r.Start()
	require.ErrorIs(t, err, syncjob.ErrAlreadyRunning)

	r.Cancel()
	waitForStatus(t, r, syncjob.StatusCancelled)
}

func TestRunner_Cancel_StopsBeforeRemainingAccounts(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{
		mustAccount(t, "acc-1", "Konto 1"),
		mustAccount(t, "acc-2", "Konto 2"),
	}}
	syncer := &fakeSyncer{block: true}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	// Sicherstellen, dass der Lauf tatsächlich am ersten (blockierenden)
	// Konto hängt, bevor abgebrochen wird.
	require.Eventually(t, func() bool { return syncer.callCount() == 1 }, time.Second, 5*time.Millisecond)

	r.Cancel()
	final := waitForStatus(t, r, syncjob.StatusCancelled)

	require.Equal(t, 1, syncer.callCount(), "das zweite Konto darf nach dem Abbruch nicht mehr angefasst werden")
	require.Equal(t, syncjob.StatusCancelled, final.Status)
}

// TestRunner_Cancel_DuringOnlyAccount_StillReportsCancelled ist eine
// Regression: Cancel() während des LETZTEN (hier: einzigen) Kontos
// erreicht die "cancelled = true; break"-Prüfung am Schleifenanfang
// nicht mehr (es gibt kein weiteres Konto, bei dem die Prüfung noch
// laufen würde) — ohne eine zusätzliche Prüfung nach der Schleife wurde
// ein währenddessen abgebrochener Lauf fälschlich als "done" gemeldet.
func TestRunner_Cancel_DuringOnlyAccount_StillReportsCancelled(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Konto 1")}}
	syncer := &fakeSyncer{block: true}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	require.Eventually(t, func() bool { return syncer.callCount() == 1 }, time.Second, 5*time.Millisecond)

	r.Cancel()
	final := waitForStatus(t, r, syncjob.StatusCancelled)

	require.Equal(t, syncjob.StatusCancelled, final.Status)
	require.NoError(t, final.Err, "ein gewollter Abbruch ist kein Fehler")
}

func TestRunner_Cancel_WithoutRunningJob_DoesNothing(t *testing.T) {
	t.Parallel()

	r := syncjob.NewRunner(context.Background(), &fakeAccountRepository{}, &fakeSyncer{})
	require.NotPanics(t, r.Cancel)
	require.Equal(t, syncjob.StatusIdle, r.Snapshot().Status)
}

func TestRunner_AccountListError_SetsStatusFailed(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{err: errors.New("datenbank kaputt")}
	r := syncjob.NewRunner(context.Background(), accounts, &fakeSyncer{})

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusFailed)
	require.Error(t, final.Err)
}

func TestRunner_SingleAccountError_ContinuesWithRemainingAccounts(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{
		mustAccount(t, "acc-1", "Konto 1"),
		mustAccount(t, "acc-2", "Konto 2"),
	}}
	syncer := &fakeSyncer{err: errors.New("verbindung fehlgeschlagen")}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount(), "ein fehlerhaftes Konto darf die übrigen nicht blockieren")
	require.Error(t, final.Err)
}

func TestRunner_Subscribe_ReceivesProgressUpdates(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Konto 1")}}
	syncer := &controlledProgressSyncer{proceed: make(chan struct{})}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	ch, unsubscribe := r.Subscribe()
	defer unsubscribe()

	require.NoError(t, r.Start())

	// syncer.SyncAccount blockiert nach der Fortschrittsmeldung, bis wir
	// proceed schließen — der Zwischenstand kann also nicht von einem
	// späteren Broadcast überschrieben werden, bevor wir ihn lesen.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case s := <-ch:
			if s.Progress.Processed == 1 {
				close(syncer.proceed)
				waitForStatus(t, r, syncjob.StatusDone)
				return
			}
		case <-deadline:
			t.Fatal("kein Fortschritts-Zwischenstand innerhalb der Frist beim Zuhörer angekommen")
		}
	}
}

func TestRunner_Subscribe_NewSubscriberGetsCurrentStateImmediately(t *testing.T) {
	t.Parallel()

	r := syncjob.NewRunner(context.Background(), &fakeAccountRepository{}, &fakeSyncer{})

	ch, unsubscribe := r.Subscribe()
	defer unsubscribe()

	select {
	case s := <-ch:
		require.Equal(t, syncjob.StatusIdle, s.Status)
	default:
		t.Fatal("ein neuer Zuhörer muss sofort den aktuellen Stand bekommen")
	}
}

func TestRunner_SecondRun_AfterCompletion_Works(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Konto 1")}}
	syncer := &fakeSyncer{result: syncreports.Result{New: 1}}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	waitForStatus(t, r, syncjob.StatusDone)

	require.NoError(t, r.Start())
	waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount())
}
