// Package syncreports orchestrates the "fetch and import reports" use
// case (IMPLEMENTIERUNG.md section 7): connect, fetch new messages,
// decode MIME, parse, deduplicate, save, persist progress. Only knows
// domain ports, no concrete infra.
package syncreports

import (
	"context"
	"fmt"
	stdsync "sync" // aliased: internal/domain/sync is also called "sync"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// defaultConcurrency caps parallel parser workers when UseCase.
// Concurrency is unset. MIME decoding and XML parsing are CPU-bound
// (IMPLEMENTIERUNG.md section 7.3) — more than a handful of workers
// achieves nothing without a corresponding number of cores.
const defaultConcurrency = 4

// UseCase orchestrates SyncAccount. All fields are ports — the
// composition root (cmd/dmarc-analyzer) wires up the concrete adapters.
type UseCase struct {
	Accounts account.Repository
	// Secret is the IMAP password loaded from ENV (see
	// internal/infra/envconfig) — valid for the entire process lifetime,
	// there is only the one configured account.
	Secret        account.Secret
	States        domainsync.StateRepository
	Reports       report.Repository
	FailedImports domainsync.FailedImportRepository
	Decoder       domainsync.MessageDecoder
	// Parsers is searched in order for the first Supports() match
	// (open/closed: RUF/TLS-RPT parsers are added additively).
	Parsers []domainsync.ReportParser
	// NewSource returns a fresh, unconnected MessageSource for each run —
	// a factory instead of a shared instance, because each run builds its
	// own connection and closes it afterward.
	NewSource func() domainsync.MessageSource
	// Concurrency caps parallel parser workers. 0 → defaultConcurrency.
	Concurrency int
}

// Result summarizes a sync run
// (IMPLEMENTIERUNG.md section 7.1, step 7: "new / skipped / failed").
type Result struct {
	New     int
	Skipped int
	Failed  int
	// Errors are detailed errors for failed messages/reports — alongside
	// Failed, not instead of it (multiple attachments of one message can
	// fail independently).
	Errors []error
}

// Progress is the intermediate state of a running sync — the same
// counters as Result, but reported after each completed message instead
// of only at the end (MIGRATIONSPLAN.md extension 9.1: "progress
// callback (processed / new / skipped / failed)"). Processed counts
// messages (a message can have multiple reports as attachments, which
// individually feed into New/Skipped/Failed), New/Skipped/Failed count
// reports as in Result.
type Progress struct {
	Processed int
	New       int
	Skipped   int
	Failed    int
}

// OnProgress is called — if not nil — after every completed message with
// the cumulative intermediate state. Runs on the serial writer goroutine
// (see write()), never concurrently — a caller needs no synchronization
// of its own, but must not block itself (otherwise it blocks the entire
// sync).
type OnProgress func(Progress)

// SyncAccount performs an incremental sync for accountID. onProgress is
// optional (nil: no progress reported) — the web UI hooks in the SSE
// sender here (internal/app/syncjob), the CLI and the Fyne UI pass nil
// (MIGRATIONSPLAN.md extension 9.1).
func (uc *UseCase) SyncAccount(ctx context.Context, accountID account.AccountID, onProgress OnProgress) (Result, error) {
	acc, err := uc.Accounts.FindByID(ctx, accountID)
	if err != nil {
		return Result{}, fmt.Errorf("account %q could not be loaded: %w", accountID, err)
	}

	source := uc.NewSource()
	defer func() { _ = source.Close() }()

	if err := source.Connect(ctx, *acc, uc.Secret); err != nil {
		return Result{}, fmt.Errorf("could not connect to account %q: %w", accountID, err)
	}

	state, err := uc.States.Load(ctx, accountID, acc.Mailbox)
	if err != nil {
		return Result{}, fmt.Errorf("sync progress for account %q could not be loaded: %w", accountID, err)
	}

	seq, baseline, err := source.FetchNew(ctx, state)
	if err != nil {
		return Result{}, fmt.Errorf("could not fetch new messages for account %q: %w", accountID, err)
	}

	// Persist the baseline immediately: even with zero processed
	// messages, e.g. UIDValidity may have changed — that must be
	// recorded before any message has been processed at all.
	if err := uc.States.Save(ctx, baseline); err != nil {
		return Result{}, fmt.Errorf("sync progress for account %q could not be saved: %w", accountID, err)
	}

	return uc.runPipeline(ctx, baseline, seq, onProgress)
}

// fetchResult carries either a fetched message or the (one-time, final)
// error of the iterator to the worker pool.
type fetchResult struct {
	msg domainsync.RawMessage
	err error
}

// parseResult carries the result of a parser worker back to the serial
// writer.
type parseResult struct {
	msg     domainsync.RawMessage
	reports []*report.AggregateReport
	err     error
	// isFetchErr distinguishes a fetch/iterator error (not tied to a
	// specific message) from a processing error of an actually fetched
	// message.
	isFetchErr bool
}

// runPipeline implements fetching/parsing concurrently, writing serially
// (IMPLEMENTIERUNG.md section 7.3): a fetcher goroutine reads from seq, a
// worker pool decodes and parses in parallel (order not guaranteed), the
// caller itself writes serially and only persists progress via a gapless
// boundary (progressTracker) — this stays correct even with messages
// completed out of order.
func (uc *UseCase) runPipeline(ctx context.Context, baseline domainsync.State, seq func(func(domainsync.RawMessage, error) bool), onProgress OnProgress) (Result, error) {
	jobs := make(chan fetchResult)
	results := make(chan parseResult)

	go uc.fetch(ctx, seq, jobs)

	concurrency := uc.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	var wg stdsync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uc.parseWorker(ctx, jobs, results)
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	return uc.write(ctx, baseline, results, onProgress)
}

func (uc *UseCase) fetch(ctx context.Context, seq func(func(domainsync.RawMessage, error) bool), jobs chan<- fetchResult) {
	defer close(jobs)

	for msg, err := range seq {
		select {
		case jobs <- fetchResult{msg: msg, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return // Iterator error is final, see fetch.go in internal/infra/imap.
		}
	}
}

func (uc *UseCase) parseWorker(ctx context.Context, jobs <-chan fetchResult, results chan<- parseResult) {
	for job := range jobs {
		var res parseResult
		if job.err != nil {
			res = parseResult{err: job.err, isFetchErr: true}
		} else {
			reports, err := uc.decodeAndParse(ctx, job.msg)
			res = parseResult{msg: job.msg, reports: reports, err: err}
		}

		select {
		case results <- res:
		case <-ctx.Done():
			return
		}
	}
}

// decodeAndParse splits a message into attachments and parses each
// supported attachment. Unsupported attachments (Supports() == false,
// e.g. the text body or an embedded logo) are skipped, not treated as an
// error.
func (uc *UseCase) decodeAndParse(ctx context.Context, msg domainsync.RawMessage) ([]*report.AggregateReport, error) {
	attachments, err := uc.Decoder.Decode(msg.Data)
	if err != nil {
		return nil, fmt.Errorf("message could not be decoded: %w", err)
	}

	var reports []*report.AggregateReport
	for _, att := range attachments {
		parser := uc.findParser(att)
		if parser == nil {
			continue
		}

		// ParseAttachment uses ParseAll if the parser additionally
		// implements it (e.g. dmarcxml.Parser for a .zip attachment with
		// multiple XML files) — a single attachment can thus yield
		// multiple reports (see domainsync.ParseAttachment
		// documentation).
		parsed, err := domainsync.ParseAttachment(ctx, parser, att)
		if err != nil {
			return reports, fmt.Errorf("attachment %q could not be parsed: %w", att.Filename, err)
		}
		reports = append(reports, parsed...)
	}
	return reports, nil
}

func (uc *UseCase) findParser(att domainsync.RawAttachment) domainsync.ReportParser {
	for _, p := range uc.Parsers {
		if p.Supports(att) {
			return p
		}
	}
	return nil
}

// write is the only goroutine that saves reports and persists progress —
// serially, as required by IMPLEMENTIERUNG.md section 7.3 ("SQLite
// doesn't like concurrent writers").
func (uc *UseCase) write(ctx context.Context, baseline domainsync.State, results <-chan parseResult, onProgress OnProgress) (Result, error) {
	result := Result{}
	state := baseline
	tracker := newProgressTracker(baseline.LastUID)
	processed := 0

	for res := range results {
		if res.isFetchErr {
			result.Errors = append(result.Errors, res.err)
			continue
		}

		processed++

		if res.err != nil {
			result.Failed++
			result.Errors = append(result.Errors, res.err)
			if uc.FailedImports != nil {
				_ = uc.FailedImports.Record(ctx, domainsync.FailedImport{
					AccountID:  baseline.AccountID,
					MessageUID: res.msg.UID,
					Error:      res.err.Error(),
					Raw:        res.msg.Data,
					OccurredAt: time.Now(),
				})
			}
		} else {
			uc.saveReports(ctx, res.reports, &result)
		}

		if err := uc.advanceProgress(ctx, tracker, &state, res.msg.UID); err != nil {
			result.Errors = append(result.Errors, err)
		}

		if onProgress != nil {
			onProgress(Progress{
				Processed: processed,
				New:       result.New,
				Skipped:   result.Skipped,
				Failed:    result.Failed,
			})
		}
	}

	return result, nil
}

func (uc *UseCase) saveReports(ctx context.Context, reports []*report.AggregateReport, result *Result) {
	for _, rep := range reports {
		imported, err := report.SaveIfNew(ctx, uc.Reports, rep)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("report could not be saved: %w", err))
			continue
		}
		if imported {
			result.New++
		} else {
			result.Skipped++
		}
	}
}

// advanceProgress marks uid as completed and persists state if the
// gapless progress boundary changed as a result.
func (uc *UseCase) advanceProgress(ctx context.Context, tracker *progressTracker, state *domainsync.State, uid uint32) error {
	lastUID, advanced := tracker.markDone(uid)
	if !advanced {
		return nil
	}

	state.LastUID = lastUID
	state.LastSyncAt = time.Now()
	if err := uc.States.Save(ctx, *state); err != nil {
		// Progress could not be saved — in the next run, individual
		// messages will at most be reprocessed, the UNIQUE index catches
		// any resulting duplicates (IMPLEMENTIERUNG.md section 7.2). This
		// doesn't abort the run, but is visible in the Result (see
		// write()).
		return fmt.Errorf("sync progress could not be saved: %w", err)
	}
	return nil
}
