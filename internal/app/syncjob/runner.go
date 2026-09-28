// Package syncjob orchestrates a sync run across all configured accounts
// as a server-side background job: at most one run at a time, cancelable
// via context, progress for any number of listeners (MIGRATIONSPLAN.md
// extension 9.2). Same flow as previously internal/ui/app.go
// (shell.startSync): all accounts one after another, results summed up,
// one shared cancellation — here server-side, because the sync must keep
// running beyond the single HTTP request that triggered it (page
// navigation, multiple browser tabs, MIGRATIONSPLAN.md section 8: "sync
// keeps running server-side").
package syncjob

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// Syncer is the slice of syncreports.UseCase that Runner needs — its own
// interface instead of a concrete struct dependency, so tests can use a
// fake instead of a fully wired UseCase (with IMAP/decoder/parser fakes).
type Syncer interface {
	SyncAccount(ctx context.Context, id account.AccountID, onProgress syncreports.OnProgress) (syncreports.Result, error)
}

var _ Syncer = (*syncreports.UseCase)(nil)

// ErrAlreadyRunning is reported by Start() when a run is already active.
var ErrAlreadyRunning = errors.New("a sync is already running")

// Status is the coarse state of a sync run.
type Status string

// Values for Status.
const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
	StatusFailed    Status = "failed"
)

// State is a snapshot of the current or last sync run — distributed to
// every listener (Subscribe).
type State struct {
	Status Status

	TotalAccounts  int
	AccountIndex   int    // 1-based, 0 outside a run
	CurrentAccount string // DisplayName of the currently running account, empty outside a run

	// Progress is the intermediate state of the currently running account
	// — reset for every new account (not a cumulative counter across
	// multiple accounts, see Total for that).
	Progress syncreports.Progress
	// Total is only set once the entire run has ended (Done/Cancelled/
	// Failed) — the sum across all completed accounts.
	Total syncreports.Result

	// Err is set when Status is Failed (e.g. the account list couldn't be
	// loaded). An error on a SINGLE account doesn't lead to Failed — the
	// run continues with the remaining accounts, as previously in
	// internal/ui/app.go (called "firstErr" there) — the first account
	// error still ends up here, for display after the run ends.
	Err error

	StartedAt time.Time
	EndedAt   time.Time
}

// Runner runs at most one sync run at a time.
type Runner struct {
	baseCtx  context.Context
	accounts account.Repository
	sync     Syncer

	mu          sync.Mutex
	running     bool
	cancel      context.CancelFunc
	state       State
	subscribers map[chan State]struct{}
}

// NewRunner creates a Runner. baseCtx determines the maximum lifetime of
// a run (in production the server lifecycle, see cmd_web.go) — NOT the
// context of the individual HTTP request that triggers the run via
// Start().
func NewRunner(baseCtx context.Context, accounts account.Repository, sync Syncer) *Runner {
	return &Runner{
		baseCtx:     baseCtx,
		accounts:    accounts,
		sync:        sync,
		state:       State{Status: StatusIdle},
		subscribers: make(map[chan State]struct{}),
	}
}

// Start triggers a new run across all configured accounts.
// ErrAlreadyRunning if one is already running.
func (r *Runner) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(r.baseCtx)
	r.running = true
	r.cancel = cancel
	r.state = State{Status: StatusRunning, StartedAt: time.Now()}
	r.broadcastLocked()
	r.mu.Unlock()

	go r.run(ctx)
	return nil
}

// Cancel aborts a running sync — not an error if none is running.
func (r *Runner) Cancel() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Snapshot returns the current state without subscribing.
func (r *Runner) Snapshot() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Subscribe returns a channel that receives every state change from now
// on — starting with the current state, so a newly joining listener
// (e.g. a second browser tab) doesn't have to wait for the next change
// first. The channel is buffered (size 1) and, on a not-yet-read, stale
// state, discards it in favor of the newest — a slow or disconnected
// listener must never throttle the sync itself.
func (r *Runner) Subscribe() (<-chan State, func()) {
	ch := make(chan State, 1)

	r.mu.Lock()
	r.subscribers[ch] = struct{}{}
	ch <- r.state
	r.mu.Unlock()

	unsubscribe := func() {
		r.mu.Lock()
		delete(r.subscribers, ch)
		r.mu.Unlock()
	}
	return ch, unsubscribe
}

// broadcastLocked distributes r.state to all listeners — must be called
// with r.mu held.
func (r *Runner) broadcastLocked() {
	for ch := range r.subscribers {
		select {
		case ch <- r.state:
		default:
			// Channel is full: discard the oldest, not-yet-read state and
			// deliver the newest instead, rather than blocking.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- r.state:
			default:
			}
		}
	}
}

func (r *Runner) setState(mutate func(*State)) {
	r.mu.Lock()
	mutate(&r.state)
	r.broadcastLocked()
	r.mu.Unlock()
}

func (r *Runner) run(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		r.running = false
		r.cancel = nil
		r.mu.Unlock()
	}()

	accounts, err := r.accounts.FindAll(ctx)
	if err != nil {
		r.setState(func(s *State) {
			s.Status = StatusFailed
			s.Err = err
			s.EndedAt = time.Now()
		})
		return
	}

	r.setState(func(s *State) { s.TotalAccounts = len(accounts) })

	total := syncreports.Result{}
	var firstErr error
	cancelled := false

	for i, acc := range accounts {
		if ctx.Err() != nil {
			cancelled = true
			break
		}

		index := i + 1
		accountID := acc.ID
		r.setState(func(s *State) {
			s.AccountIndex = index
			s.CurrentAccount = acc.DisplayName
			s.Progress = syncreports.Progress{}
		})

		result, syncErr := r.sync.SyncAccount(ctx, accountID, func(p syncreports.Progress) {
			r.setState(func(s *State) { s.Progress = p })
		})
		total.New += result.New
		total.Skipped += result.Skipped
		total.Failed += result.Failed
		total.Errors = append(total.Errors, result.Errors...)
		if syncErr != nil && firstErr == nil {
			firstErr = syncErr
		}
	}

	status := StatusDone
	// Check ctx.Err() in addition to "cancelled": cancelled is only set
	// before the NEXT account (line above) — if the context is canceled
	// exactly during the last (or only) account, the loop exits
	// afterward completely normally, without "cancelled" ever being set.
	// Without this second check, a run canceled in the middle of the
	// only account would be wrongly reported as "done".
	if cancelled || ctx.Err() != nil {
		status = StatusCancelled
		// A ctx.Canceled/DeadlineExceeded triggered by Cancel() is the
		// EXPECTED cause of the cancellation, not a standalone error —
		// otherwise the UI would show "cancelled" AND an error message at
		// the same time for the same, intentional action.
		firstErr = nil
	}
	r.setState(func(s *State) {
		s.Status = status
		s.Total = total
		s.Err = firstErr
		s.AccountIndex = 0
		s.CurrentAccount = ""
		s.EndedAt = time.Now()
	})
}
