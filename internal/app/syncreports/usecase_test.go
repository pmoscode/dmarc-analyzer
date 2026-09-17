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
	errTest    = errors.New("testfehler")
)

const testAccountID account.AccountID = "acc-1"

func testAccount(t *testing.T) account.MailAccount {
	t.Helper()
	acc, err := account.NewMailAccount(testAccountID, "Test", "imap.example.com", 993, "user@example.com", "INBOX", true, time.Now())
	require.NoError(t, err)
	return *acc
}

// newTestUseCase baut einen UseCase mit Fakes, die per Parameter
// überschrieben werden können.
type testDeps struct {
	accounts      *fakeAccountRepository
	credentials   *fakeCredentialStore
	states        *fakeStateRepository
	reports       *fakeReportRepository
	failedImports *fakeFailedImportRepository
	source        *fakeMessageSource
}

func newTestUseCase(t *testing.T, messages []domainsync.RawMessage, parserFailFor map[string]error) (*syncreports.UseCase, *testDeps) {
	t.Helper()

	deps := &testDeps{
		accounts:      newFakeAccountRepository(testAccount(t)),
		credentials:   newFakeCredentialStore(),
		states:        newFakeStateRepository(),
		reports:       newFakeReportRepository(),
		failedImports: newFakeFailedImportRepository(),
		source:        &fakeMessageSource{messages: messages},
	}
	require.NoError(t, deps.credentials.Store(testAccountID, account.NewSecretFromString("app-passwort")))

	uc := &syncreports.UseCase{
		Accounts:      deps.accounts,
		Credentials:   deps.credentials,
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
	require.Equal(t, uint32(3), state.LastUID, "nach vollständigem Lauf muss LastUID auf die höchste UID zeigen")
	require.True(t, deps.source.closed, "MessageSource muss nach dem Lauf geschlossen werden")
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

	require.Len(t, updates, 3, "ein Aufruf je verarbeiteter Nachricht")
	// Reihenfolge der Nachrichten ist nicht garantiert (nebenläufige
	// Parser-Worker, siehe runPipeline-Dokumentation) — deshalb nur die
	// letzte, kumulierte Momentaufnahme prüfen, nicht jeden Zwischenwert.
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
	// Der zentrale End-to-End-Fall über MessageSource hinaus: derselbe
	// Report kommt (z. B. durch eine erneut zugestellte Mail) ein zweites
	// Mal an — Exists()/ErrDuplicate sorgen dafür, dass er als "skipped"
	// gezählt wird, nicht als Fehler.
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{msg(1, "report-a")}, nil)

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	// Zweiter Lauf: MessageSource liefert dieselbe Nachricht erneut (z. B.
	// weil die Postfach-UIDVALIDITY sich geändert hätte).
	deps.source.messages = []domainsync.RawMessage{msg(1, "report-a")}
	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	require.Zero(t, result.New)
	require.Equal(t, 1, result.Skipped)
	require.Equal(t, 1, deps.reports.count(), "darf nicht dupliziert werden")
}

func TestSyncAccount_ParseError_CountsAsFailedAndQuarantines(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "kaputter-report"),
	}, map[string]error{"kaputter-report": errTest})

	result, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.NoError(t, err)

	require.Equal(t, 1, result.New)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 1, deps.failedImports.count(), "fehlgeschlagener Import muss in die Quarantäne")

	state, err := deps.states.Load(context.Background(), testAccountID, "INBOX")
	require.NoError(t, err)
	require.Equal(t, uint32(2), state.LastUID,
		"eine dauerhaft fehlschlagende Nachricht darf den Fortschritt nicht für immer blockieren")
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

	// Baseline wird sofort gespeichert, auch ganz ohne Nachrichten (siehe
	// UseCase.SyncAccount) — relevant, wenn sich nur die UIDValidity
	// geändert hat.
	require.NotEmpty(t, deps.states.saves)
}

func TestSyncAccount_ConnectFailure_ReturnsErrorWithoutPartialState(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, nil, nil)
	deps.source.connectErr = errTest

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.Error(t, err)
	require.Empty(t, deps.states.saves, "bei fehlgeschlagener Verbindung darf kein Fortschritt gespeichert werden")
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
	_, err := uc.SyncAccount(context.Background(), "nie-angelegt", nil)
	require.Error(t, err)
}

func TestSyncAccount_MissingCredentials_ReturnsError(t *testing.T) {
	t.Parallel()

	uc, deps := newTestUseCase(t, nil, nil)
	require.NoError(t, deps.credentials.Delete(testAccountID))

	_, err := uc.SyncAccount(context.Background(), testAccountID, nil)
	require.Error(t, err)
}

func TestSyncAccount_ContextCancelledMidSync_LeavesConsistentState(t *testing.T) {
	// UMSETZUNGSPLAN.md AP 4: "Abbruch per context.Cancel hinterlässt
	// konsistenten Zustand." Konsistent heißt hier: LastUID zeigt nie auf
	// eine Nachricht, die nicht tatsächlich (erfolgreich oder in die
	// Quarantäne) verarbeitet wurde, und bereits gespeicherte Reports
	// bleiben unangetastet.
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	uc, deps := newTestUseCase(t, []domainsync.RawMessage{
		msg(1, "report-a"),
		msg(2, "report-b"),
		msg(3, "report-c"),
	}, nil)
	// Nach der ersten Nachricht abbrechen, indem der Fake beim Iterieren
	// selbst den Kontext prüft (siehe fakeMessageSource.FetchNew) — cancel
	// wird über einen Parser-Seiteneffekt ausgelöst: der erste geparste
	// Report löst den Abbruch aus.
	uc.Parsers = []domainsync.ReportParser{cancelingParser{parser: fakeParser{}, cancel: cancel}}
	uc.Concurrency = 1 // deterministische Reihenfolge für diesen Test

	result, err := uc.SyncAccount(ctx, testAccountID, nil)
	require.NoError(t, err, "SyncAccount selbst meldet den Abbruch über Result.Errors, nicht als Rückgabefehler")

	state, loadErr := deps.states.Load(context.Background(), testAccountID, "INBOX")
	require.NoError(t, loadErr)

	// Der entscheidende Konsistenz-Check: LastUID darf niemals eine UID
	// überspringen, die nicht tatsächlich verarbeitet wurde.
	require.LessOrEqual(t, int(state.LastUID), deps.reports.count()+result.Failed,
		"LastUID darf nicht weiter fortgeschrieben sein, als tatsächlich verarbeitete Nachrichten existieren")

	// Für jede als "neu" gezählte Nachricht muss auch wirklich ein Report
	// gespeichert sein — kein Zählen ohne tatsächliche Persistenz.
	require.Equal(t, result.New, deps.reports.count())
}

// cancelingParser ruft nach dem ersten erfolgreichen Parse cancel() auf —
// simuliert einen Nutzer, der den Sync mitten im Lauf abbricht.
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
