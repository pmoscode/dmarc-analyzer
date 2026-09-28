// Package sync contains the sync progress (State) and the ports
// MessageSource and ReportParser (IMPLEMENTIERUNG.md section 6.4).
package sync

import (
	"context"
	"iter"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// RawAttachment is an unprocessed attachment from a source: raw bytes plus
// the metadata needed for format detection.
type RawAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// MessageDecoder splits a raw message (RawMessage.Data) into its
// attachments — the protocol-independent step between MessageSource and
// ReportParser (see comment on RawMessage). Implemented in work package 4
// against github.com/emersion/go-message (internal/infra/mailmime). Its
// own port instead of a direct import of internal/infra/mailmime from the
// application layer — otherwise syncreports would depend on a concrete
// infra technology instead of a port (DIP, IMPLEMENTIERUNG.md section
// 4.2).
type MessageDecoder interface {
	Decode(data []byte) ([]RawAttachment, error)
}

// ReportParser converts a raw attachment into domain objects. Supports()
// selects the matching implementation — this way RUF and TLS-RPT can be
// added additively later without changing existing code (open/closed,
// IMPLEMENTIERUNG.md section 6.4).
type ReportParser interface {
	Supports(attachment RawAttachment) bool
	Parse(ctx context.Context, attachment RawAttachment) (*report.AggregateReport, error)
}

// MultiReportParser is an optional extension of ReportParser for
// attachments that can contain multiple reports — for DMARC aggregate
// reports in particular a .zip with several XML files
// (internal/infra/dmarcxml.Parser already implements this via ParseAll).
// Callers (importfiles/syncreports) check via type assertion whether a
// parser implements this interface in addition to ReportParser — the
// same optional-interface slicing as MailboxLister further below. A
// parser that only implements Parse() then simply returns the single
// report that Parse() returns — not an error, just less functionality.
type MultiReportParser interface {
	ParseAll(ctx context.Context, attachment RawAttachment) ([]*report.AggregateReport, error)
}

// ParseAttachment returns all reports contained in attachment —
// preferably via MultiReportParser.ParseAll if parser additionally
// implements that (e.g. for .zip attachments with multiple XML files),
// otherwise via the single Parse() from ReportParser. Shared by
// importfiles.UseCase and syncreports.UseCase so both behave the same way
// (before this function, importfiles imported a .zip with multiple
// reports only incompletely — only the first contained report was ever
// saved, see regression test
// TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth in
// internal/web).
func ParseAttachment(ctx context.Context, parser ReportParser, attachment RawAttachment) ([]*report.AggregateReport, error) {
	if multi, ok := parser.(MultiReportParser); ok {
		return multi.ParseAll(ctx, attachment)
	}
	rep, err := parser.Parse(ctx, attachment)
	if err != nil {
		return nil, err
	}
	return []*report.AggregateReport{rep}, nil
}

// RawMessage is an unprocessed message from a source: the complete raw
// bytes (for IMAP: BODY.PEEK[], for file import (work package 4/7): the
// file content) plus the UID under which the source keeps it. The MIME
// decomposition into RawAttachment values deliberately does NOT happen
// here, but as its own, protocol-independent step in the application
// layer (work package 4) — the same logic then processes both IMAP
// messages and imported .eml files.
type RawMessage struct {
	UID  uint32
	Data []byte
}

// MessageSource returns raw messages from a source (v1: IMAP,
// internal/infra/imap). Deliberately technology-neutral, so that file
// import or other protocols can be added later without changing the use
// cases (IMPLEMENTIERUNG.md section 6.4).
//
// Diverging from the original design, Connect takes the Secret
// separately instead of reading it from account.MailAccount: per the
// security model (IMPLEMENTIERUNG.md section 9), MailAccount deliberately
// contains no password, which lives exclusively in the credential store.
// The caller fetches it via account.CredentialStore and passes it through
// explicitly here.
type MessageSource interface {
	Connect(ctx context.Context, acc account.MailAccount, secret account.Secret) error
	// FetchNew returns an iterator over new messages since state, plus a
	// baseline state that can be persisted immediately (in particular the
	// current UIDValidity) — even before the iterator has been consumed.
	// The caller itself persists the actual progress (LastUID) after each
	// successfully processed RawMessage (IMPLEMENTIERUNG.md section 7.1,
	// step 6) — FetchNew supplies that UID with every RawMessage for this
	// purpose.
	FetchNew(ctx context.Context, state State) (iter.Seq2[RawMessage, error], State, error)
	Close() error
}

// MailboxLister is an optional additional port for sources that can list
// the mailboxes/folders actually present on the server — the basis for a
// folder picker in the account form, since DMARC reports don't
// necessarily end up in the root mailbox (INBOX), but can e.g. be sorted
// into a subfolder via a mail rule. Deliberately its own, separate port
// instead of another MessageSource method: not every conceivable source
// (e.g. a future file adapter) can meaningfully provide this — callers
// check via type assertion whether a MessageSource additionally
// implements MailboxLister (see manageaccount.UseCase.ListMailboxes).
//
// Like FetchNew, requires an already successful Connect() connection.
type MailboxLister interface {
	ListMailboxes(ctx context.Context) ([]string, error)
}
