package sync

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// FailedImport is a message (or an attachment within it) that couldn't be
// processed — ends up in the error quarantine instead of aborting the
// whole sync run (IMPLEMENTIERUNG.md section 7.2). Viewable in the UI and
// individually retriable (work package 5/7).
type FailedImport struct {
	AccountID  account.AccountID
	MessageUID uint32
	Filename   string
	Error      string
	// Raw is the unprocessed attachment or message — allows re-parsing
	// after a parser fix without querying the mailbox again.
	Raw        []byte
	OccurredAt time.Time
}

// FailedImportRepository is the port for the error quarantine. Implemented
// in work package 4 against SQLite (table failed_imports, schema from
// work package 2).
type FailedImportRepository interface {
	Record(ctx context.Context, f FailedImport) error
}
