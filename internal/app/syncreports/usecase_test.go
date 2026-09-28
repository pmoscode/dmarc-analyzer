package syncreports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

var (
	fixedBegin = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	hour       = time.Hour
	errTest    = errors.New("test error")
)

const testAccountID account.AccountID = "acc-1"

func testAccount(t *testing.T) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(testAccountID, "Test", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return *acc
}

// newTestUseCase builds a UseCase with fakes that can be overridden via
// parameters.
type testDeps struct {
	accounts      *fakeAccountRepository
	states        *fakeStateRepository
	reports       *fakeReportRepository
	failedImports *fakeFailedImportRepository
	source        *fakeMessageSource
}

func newTestUseCase(t *testing.T, messages []domainsync.RawMessage, parserFailFor map[string]error) (*syncreports.UseCase, *testDeps) {
	t.Helper()

	deps := &testDeps{
		accounts:      newFakeAccountRepository(testAccount(t)),
		states:        newFakeStateRepository(),
		reports:       newFakeReportRepository(),
		failedImports: newFakeFailedImportRepository(),
		source:        &fakeMessageSource{messages: messages},
	}

	uc := &syncreports.UseCase{
		Accounts:      deps.accounts,
		Secret:        account.NewSecretFromString("app-password"),
		States:        deps.states,
		Reports:       deps.reports,
		FailedImports: deps.failedImports,
		Decoder:       fakeDecoder{},
		Parsers:       []domainsync.ReportParser{fakeParser{failFor: parserFailFor}},
		NewSource:     func() domainsync.MessageSource { return deps.source },
	}
	return uc, deps
}

func msg(uid uint32, content string) domainsync.RawMessage {
	return domainsync.RawMessage{UID: uid, Data: []byte(content)}
}

func TestSyncAccount_HappyPath_ImportsAllNewReports(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "report-b"),
		msg(3, "report-c"),
	}, nil)

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	require.Equal(t, 3, result.New)
	require.Zero(t, result.Skipped)
	require.Zero(t, result.Failed)
	require.Empty(t, result.Errors)
	require.Equal(t, 3, deps.reports.count())

	state, err := deps.states.Load(context.Background(), testAccountID, "INBOX")
	require.NoError(t, err)
	require.Equal(t, uint32(3), state.LastUID, "after a complete run, LastUID must point to the highest UID")
	require.True(t, deps.source.closed, "MessageSource must be closed after the run")
}

func TestSyncAccount_OnProgress_ReportsCumulativeCountsAfterEachMessage(t *testing.T) {
	t.Parallel()

	uc, _ := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "report-b"),
		msg(3, "report-c"),
	}, nil)

	var updates []syncreports.Progress
	_, err := uc.SyncAccount(context.Background(), testAccountID, func(p syncreports.Progress) {
		updates = append(updates, p)
	})
	require.NoError(t, err)

	require.Len(t, updates, 3, "one call per processed message")
	// Message order is not guaranteed (concurrent parser workers, see
	// runPipeline documentation) — so only check the last, cumulative
	// snapshot, not every intermediate value.
	last := updates[len(updates)-1]
	require.Equal(t, 3, last.Processed)
	require.Equal(t, 3, last.New)
	require.Zero(t, last.Skipped)
	require.Zero(t, last.Failed)
}

func TestSyncAccount_NilOnProgress_DoesNotPanic(t *testing.T) {
	t.Parallel()

	uc, _ := newTestUseCase(t, []domainsync.RawMessage{msg(1, "report-a")}, nil)

	require.NotPanics(t, func() {
		_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
		require.NoError(t, err)
	})
}

func TestSyncAccount_SecondRun_SkipsAlreadyImportedReports(t *testing.T) {
	// The central end-to-end case beyond MessageSource: the same report
	// arrives a second time (e.g. because a mail was delivered again) —
	// Exists()/ErrDuplicate ensure it's counted as "skipped", not as an
	// error.
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "report-a")}, nil)

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	// Second run: MessageSource returns the same message again (e.g.
	// because the mailbox's UIDVALIDITY had changed).
	deps.source.messages = []domainsync.RawMessage{msg(1, "report-a")}
	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	require.Zero(t, result.New)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, deps.reports.count(), "must not be duplicated")
}

func TestSyncAccount_ParseError_CountsAsFailedAndQuarantines(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "broken-report"),
	}, map[string]error{"broken-report": errTest})

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	require.Equal(t, 1, result.New)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count(), "a failed import must go into quarantine")

	state, err := deps.states.Load(context.Background(), testAccountID, "INBOX")
	require.NoError(t, err)
	require.Equal(t, uint32(2), state.LastUID,
		"a permanently failing message must not block progress forever")
}

func TestSyncAccount_DecodeError_CountsAsFailed(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "irrelevant")}, nil)
	uc.Decoder = failingDecoder{err: errTest}

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, deps.failedImports.count())
}

func TestSyncAccount_NoNewMessages_StillPersistsBaseline(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, nil, nil)

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)
	require.Zero(t, result.New)

	// The baseline is saved immediately, even with no messages at all
	// (see UseCase.SyncAccount) — relevant when only the UIDValidity has
	// changed.
	require.NotEmpty(t, deps.states.saves)
}

func TestSyncAccount_ConnectFailure_ReturnsErrorWithoutPartialState(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, nil, nil)
	deps.source.connectErr = errTest

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.Error(t, err)
	require.Empty(t, deps.states.saves, "no progress may be saved when the connection fails")
}

func TestSyncAccount_FetchFailure_ReturnsError(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, nil, nil)
	deps.source.fetchErr = errTest

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.Error(t, err)
}

func TestSyncAccount_UnknownAccount_ReturnsError(t *testing.T) {
	t.Parallel()

	uc, _ := newTestUseCase(t, nil, nil)
	_, err := uc.SyncAccount(context.Background(), "never-created", nil)
	require.Error(t, err)
}

func TestSyncAccount_ContextCancelledMidSync_LeavesConsistentState(t *testing.T) {
	// UMSETZUNGSPLAN.md work package 4: "cancellation via context.Cancel
	// leaves a consistent state." Consistent here means: LastUID never
	// points to a message that wasn't actually processed (successfully
	// or into quarantine), and already saved reports stay untouched.
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "report-b"),
		msg(3, "report-c"),
	}, nil)
	// Cancel after the first message by having the fake check the
	// context itself while iterating (see fakeMessageSource.FetchNew) —
	// cancel is triggered via a parser side effect: the first parsed
	// report triggers the cancellation.
	uc.Parsers = []domainsync.ReportParser{cancelingParser{parser: fakeParser{}, cancel: cancel}}
	uc.Concurrency = 1 // deterministic order for this test

	result, err := uc.SyncAccount(ctx, testAccountID, nil)
	require.NoError(t, err, "SyncAccount itself reports the cancellation via Result.Errors, not as a return error")

	state, loadErr := deps.states.Load(context.Background(), testAccountID, "INBOX")
	require.NoError(t, loadErr)

	// The crucial consistency check: LastUID must never skip past a UID
	// that wasn't actually processed.
	require.LessOrEqual(t, int(state.LastUID), deps.reports.count()+result.Failed,
		"LastUID must not be advanced further than the number of actually processed messages")

	// For every message counted as "new", a report must really have been
	// saved — no counting without actual persistence.
	require.Equal(t, result.New, deps.reports.count())
}

// cancelingParser calls cancel() after the first successful parse —
// simulates a user canceling the sync mid-run.
type cancelingParser struct {
	parser fakeParser
	cancel context.CancelFunc
}

func (p cancelingParser) Supports(att domainsync.RawAttachment) bool {
	return p.parser.Supports(att)
}

func (p cancelingParser) Parse(ctx context.Context, att domainsync.RawAttachment) (*report.AggregateReport, error) {
	rep, err := p.parser.Parse(ctx, att)
	p.cancel()
	return rep, err
}

func TestSyncAccount_ExistsCheckError_CountsAsFailed(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "report-a")}, nil)
	deps.reports.existsErr = errTest

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Zero(t, result.New)
	require.Len(t, result.Errors, 1)
}

func TestSyncAccount_SaveError_NonDuplicate_CountsAsFailed(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "report-a")}, nil)
	deps.reports.saveErr = errTest

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Zero(t, result.New)
	require.Len(t, result.Errors, 1)
}

func TestSyncAccount_MultiReportParser_ImportsAllReportsFromOneMessage(t *testing.T) {
	// Regression: decodeAndParse used to call only Parse(), which per the
	// domainsync.ReportParser contract returns only ONE report — a
	// message with an attachment (e.g. a .zip) that can return multiple
	// reports via domainsync.MultiReportParser was thereby only
	// partially imported. See also internal/app/importfiles for the
	// same bug and internal/web
	// TestHandleImportSubmit_ZipWithMultipleReports_ImportsBoth for the
	// real dmarcxml.Parser.
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "multi:report-a,report-b,report-c")}, nil)
	uc.Parsers = []domainsync.ReportParser{fakeMultiParser{}}

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)
	require.Equal(t, 3, result.New)
	require.Zero(t, result.Skipped)
	require.Zero(t, result.Failed)
	require.Equal(t, 3, deps.reports.count())
}
