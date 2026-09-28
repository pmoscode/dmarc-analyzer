package sync

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// State is the progress of the incremental sync per account and mailbox
// (IMPLEMENTIERUNG.md section 7.1). Advanced after every successfully
// processed message, so that an abort costs at most one message to
// reprocess.
type State struct {
	AccountID account.AccountID
	Mailbox   string
	// UIDValidity identifies the mailbox's "generation". If it changes
	// between two syncs, a full rescan is needed — the UNIQUE index on
	// reports prevents duplicates in that case.
	UIDValidity uint32
	LastUID     uint32
	LastSyncAt  time.Time
}

// StateRepository is the port for persisting State, one entry per
// (AccountID, Mailbox). Implemented in work package 4 against SQLite
// (internal/infra/sqlite.SyncStateRepository) — deliberately defined only
// here, where the sync use case (syncreports) actually needs it for the
// first time, not speculatively already in work package 1/3.
type StateRepository interface {
	// Load returns the zero value State{AccountID: accountID, Mailbox:
	// mailbox} without an error when no progress has been saved yet — a
	// first sync is not an error case.
	Load(ctx context.Context, accountID account.AccountID, mailbox string) (State, error)
	Save(ctx context.Context, state State) error
}
