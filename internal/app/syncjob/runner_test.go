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

// fakeAccountRepository only implements what Runner needs (FindAll) —
// Save/FindByID/Delete are never called, but are part of the
// account.Repository interface.
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

// fakeSyncer implements syncjob.Syncer with configurable behavior per
// account — no fully wired syncreports.UseCase needed (would require
// IMAP/decoder/parser fakes that are irrelevant to syncjob: Runner only
// knows the Syncer interface).
type fakeSyncer struct {
	mu    sync.Mutex
	calls []account.AccountID

	result syncreports.Result
	err    error
	// block makes SyncAccount block until ctx is canceled — for
	// Cancel() tests.
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

// controlledProgressSyncer reports exactly one progress snapshot and
// then blocks until the test closes proceed — makes "does a snapshot
// actually reach the listener" deterministically testable instead of
// depending on the timing of a buffered channel (Runner.Subscribe
// deliberately only delivers the LATEST state each time, a fast producer
// can otherwise overwrite snapshots before a test reader sees them).
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
			t.Fatalf("status %q not reached within the deadline (last: %q)", want, r.Snapshot().Status)
		}
	}
}

func TestRunner_Start_SyncsAllAccountsSequentially(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{
		mustAccount(t, "acc-1", "Account 1"),
		mustAccount(t, "acc-2", "Account 2"),
	}}
	syncer := &fakeSyncer{result: syncreports.Result{New: 2, Skipped: 1}}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount())
	require.Equal(t, 4, final.Total.New) // 2 accounts × 2
	require.Equal(t, 2, final.Total.Skipped)
	require.Equal(t, 2, final.TotalAccounts)
	require.NoError(t, final.Err)
	require.False(t, final.EndedAt.IsZero())
}

func TestRunner_Start_AlreadyRunning_ReturnsError(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Account 1")}}
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
		mustAccount(t, "acc-1", "Account 1"),
		mustAccount(t, "acc-2", "Account 2"),
	}}
	syncer := &fakeSyncer{block: true}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	// Make sure the run is actually stuck on the first (blocking) account
	// before canceling.
	require.Eventually(t, func() bool { return syncer.callCount() == 1 }, time.Second, 5*time.Millisecond)

	r.Cancel()
	final := waitForStatus(t, r, syncjob.StatusCancelled)

	require.Equal(t, 1, syncer.callCount(), "the second account must not be touched after cancellation")
	require.Equal(t, syncjob.StatusCancelled, final.Status)
}

// TestRunner_Cancel_DuringOnlyAccount_StillReportsCancelled is a
// regression: Cancel() during the LAST (here: only) account no longer
// reaches the "cancelled = true; break" check at the start of the loop
// (there is no further account for which the check would still run) —
// without an additional check after the loop, a run canceled during that
// time was incorrectly reported as "done".
func TestRunner_Cancel_DuringOnlyAccount_StillReportsCancelled(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Account 1")}}
	syncer := &fakeSyncer{block: true}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	require.Eventually(t, func() bool { return syncer.callCount() == 1 }, time.Second, 5*time.Millisecond)

	r.Cancel()
	final := waitForStatus(t, r, syncjob.StatusCancelled)

	require.Equal(t, syncjob.StatusCancelled, final.Status)
	require.NoError(t, final.Err, "an intentional cancellation is not an error")
}

func TestRunner_Cancel_WithoutRunningJob_DoesNothing(t *testing.T) {
	t.Parallel()

	r := syncjob.NewRunner(context.Background(), &fakeAccountRepository{}, &fakeSyncer{})
	require.NotPanics(t, r.Cancel)
	require.Equal(t, syncjob.StatusIdle, r.Snapshot().Status)
}

func TestRunner_AccountListError_SetsStatusFailed(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{err: errors.New("database broken")}
	r := syncjob.NewRunner(context.Background(), accounts, &fakeSyncer{})

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusFailed)
	require.Error(t, final.Err)
}

func TestRunner_SingleAccountError_ContinuesWithRemainingAccounts(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{
		mustAccount(t, "acc-1", "Account 1"),
		mustAccount(t, "acc-2", "Account 2"),
	}}
	syncer := &fakeSyncer{err: errors.New("connection failed")}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	final := waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount(), "a failing account must not block the others")
	require.Error(t, final.Err)
}

func TestRunner_Subscribe_ReceivesProgressUpdates(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Account 1")}}
	syncer := &controlledProgressSyncer{proceed: make(chan struct{})}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	ch, unsubscribe := r.Subscribe()
	defer unsubscribe()

	require.NoError(t, r.Start())

	// syncer.SyncAccount blocks after reporting progress until we close
	// proceed — so the snapshot can't be overwritten by a later broadcast
	// before we read it.
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
			t.Fatal("no progress snapshot reached the listener within the deadline")
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
		t.Fatal("a new listener must get the current state immediately")
	}
}

func TestRunner_SecondRun_AfterCompletion_Works(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountRepository{accounts: []account.MailAccount{mustAccount(t, "acc-1", "Account 1")}}
	syncer := &fakeSyncer{result: syncreports.Result{New: 1}}
	r := syncjob.NewRunner(context.Background(), accounts, syncer)

	require.NoError(t, r.Start())
	waitForStatus(t, r, syncjob.StatusDone)

	require.NoError(t, r.Start())
	waitForStatus(t, r, syncjob.StatusDone)

	require.Equal(t, 2, syncer.callCount())
}
