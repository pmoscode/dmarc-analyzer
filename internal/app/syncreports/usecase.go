// Package syncreports orchestriert den Use Case "Reports abholen und
// importieren" (IMPLEMENTIERUNG.md Abschnitt 7): verbinden, neue
// Nachrichten abholen, MIME zerlegen, parsen, deduplizieren, speichern,
// Fortschritt sichern. Kennt nur Domänen-Ports, keine konkrete Infra.
package syncreports

import (
	"context"
	"fmt"
	stdsync "sync" // aliasiert: internal/domain/sync heißt ebenfalls "sync"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// defaultConcurrency begrenzt parallele Parser-Worker, wenn UseCase.
// Concurrency unbesetzt ist. MIME-Zerlegung und XML-Parsen sind
// CPU-gebunden (IMPLEMENTIERUNG.md Abschnitt 7.3) — mehr als ein paar
// Worker bringen ohne entsprechend viele Kerne nichts.
const defaultConcurrency = 4

// UseCase orchestriert SyncAccount. Alle Felder sind Ports — die
// Composition Root (cmd/dmarc-analyzer) verdrahtet die konkreten Adapter.
type UseCase struct {
	Accounts      account.Repository
	Credentials   account.CredentialStore
	States        domainsync.StateRepository
	Reports       report.Repository
	FailedImports domainsync.FailedImportRepository
	Decoder       domainsync.MessageDecoder
	// Parsers wird der Reihe nach nach dem ersten Supports()-Treffer
	// durchsucht (Open/Closed: RUF/TLS-RPT-Parser kommen additiv dazu).
	Parsers []domainsync.ReportParser
	// NewSource liefert je Lauf eine frische, unverbundene MessageSource —
	// eine Factory statt einer geteilten Instanz, weil jeder Lauf seine
	// eigene Verbindung aufbaut und danach schließt.
	NewSource func() domainsync.MessageSource
	// Concurrency begrenzt parallele Parser-Worker. 0 → defaultConcurrency.
	Concurrency int
}

// Result fasst einen Sync-Lauf zusammen
// (IMPLEMENTIERUNG.md Abschnitt 7.1, Schritt 7: "neu / übersprungen / fehlerhaft").
type Result struct {
	New     int
	Skipped int
	Failed  int
	// Errors sind Detailfehler zu fehlgeschlagenen Nachrichten/Reports —
	// gemeinsam mit Failed, nicht statt dessen (mehrere Anhänge einer
	// Nachricht können unabhängig fehlschlagen).
	Errors []error
}

// Progress ist der Zwischenstand eines laufenden Sync — dieselben
// Zähler wie Result, aber nach jeder abgeschlossenen Nachricht gemeldet
// statt erst am Ende (MIGRATIONSPLAN.md Erweiterung 9.1: "Fortschritts-
// Callback (verarbeitet / neu / übersprungen / fehlerhaft)"). Processed
// zählt Nachrichten (eine Nachricht kann mehrere Reports als Anhang
// haben, die einzeln in New/Skipped/Failed einfließen), New/Skipped/
// Failed zählen wie in Result Reports.
type Progress struct {
	Processed int
	New       int
	Skipped   int
	Failed    int
}

// OnProgress wird — falls nicht nil — nach jeder abgeschlossenen
// Nachricht mit dem kumulierten Zwischenstand aufgerufen. Läuft auf der
// seriellen Schreiber-Goroutine (siehe write()), nie nebenläufig — ein
// Aufrufer braucht keine eigene Synchronisierung, darf aber selbst nicht
// blockieren (sonst blockiert der gesamte Sync).
type OnProgress func(Progress)

// SyncAccount führt einen inkrementellen Sync für accountID durch.
// onProgress ist optional (nil: kein Fortschritt gemeldet) — die
// Web-Oberfläche hängt hier den SSE-Sender ein (internal/app/syncjob),
// die CLI und die Fyne-Oberfläche übergeben nil (MIGRATIONSPLAN.md
// Erweiterung 9.1).
func (uc *UseCase) SyncAccount(ctx context.Context, accountID account.AccountID, onProgress OnProgress) (Result, error) {
	acc, err := uc.Accounts.FindByID(ctx, accountID)
	if err != nil {
		return Result{}, fmt.Errorf("konto %q konnte nicht geladen werden: %w", accountID, err)
	}

	secret, err := uc.Credentials.Retrieve(accountID)
	if err != nil {
		return Result{}, fmt.Errorf("zugangsdaten für konto %q konnten nicht geladen werden: %w", accountID, err)
	}
	defer secret.Zero()

	source := uc.NewSource()
	defer func() { _ = source.Close() }()

	if err := source.Connect(ctx, *acc, secret); err != nil {
		return Result{}, fmt.Errorf("verbindung zu konto %q konnte nicht aufgebaut werden: %w", accountID, err)
	}

	state, err := uc.States.Load(ctx, accountID, acc.Mailbox)
	if err != nil {
		return Result{}, fmt.Errorf("sync-fortschritt für konto %q konnte nicht geladen werden: %w", accountID, err)
	}

	seq, baseline, err := source.FetchNew(ctx, state)
	if err != nil {
		return Result{}, fmt.Errorf("neue nachrichten für konto %q konnten nicht abgerufen werden: %w", accountID, err)
	}

	// Baseline sofort sichern: auch bei null verarbeiteten Nachrichten
	// kann sich z. B. UIDValidity geändert haben — das muss vermerkt sein,
	// bevor überhaupt eine Nachricht verarbeitet wurde.
	if err := uc.States.Save(ctx, baseline); err != nil {
		return Result{}, fmt.Errorf("sync-fortschritt für konto %q konnte nicht gespeichert werden: %w", accountID, err)
	}

	return uc.runPipeline(ctx, baseline, seq, onProgress)
}

// fetchResult transportiert entweder eine abgeholte Nachricht oder den
// (einmaligen, abschließenden) Fehler des Iterators zum Worker-Pool.
type fetchResult struct {
	msg domainsync.RawMessage
	err error
}

// parseResult transportiert das Ergebnis eines Parser-Workers zurück zum
// seriellen Schreiber.
type parseResult struct {
	msg     domainsync.RawMessage
	reports []*report.AggregateReport
	err     error
	// isFetchErr unterscheidet einen Fetch-/Iterator-Fehler (kein Bezug zu
	// einer bestimmten Nachricht) von einem Verarbeitungsfehler einer
	// tatsächlich abgeholten Nachricht.
	isFetchErr bool
}

// runPipeline implementiert Abholen/Parsen nebenläufig, Schreiben seriell
// (IMPLEMENTIERUNG.md Abschnitt 7.3): eine Fetcher-Goroutine liest aus
// seq, ein Worker-Pool dekodiert und parst parallel (Reihenfolge nicht
// garantiert), der Aufrufer selbst schreibt seriell und schreibt den
// Fortschritt nur über eine lückenlose Grenze fort (progressTracker) —
// das bleibt auch bei außer der Reihe abgeschlossenen Nachrichten korrekt.
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
			return // Iterator-Fehler ist abschließend, siehe fetch.go in internal/infra/imap.
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

// decodeAndParse zerlegt eine Nachricht in Anhänge und parst jeden
// unterstützten Anhang. Nicht unterstützte Anhänge (Supports() == false,
// z. B. der Textkörper oder ein Logo im Anhang) werden übersprungen, nicht
// als Fehler gewertet.
func (uc *UseCase) decodeAndParse(ctx context.Context, msg domainsync.RawMessage) ([]*report.AggregateReport, error) {
	attachments, err := uc.Decoder.Decode(msg.Data)
	if err != nil {
		return nil, fmt.Errorf("nachricht konnte nicht zerlegt werden: %w", err)
	}

	var reports []*report.AggregateReport
	for _, att := range attachments {
		parser := uc.findParser(att)
		if parser == nil {
			continue
		}

		rep, err := parser.Parse(ctx, att)
		if err != nil {
			return reports, fmt.Errorf("anhang %q konnte nicht geparst werden: %w", att.Filename, err)
		}
		reports = append(reports, rep)
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

// write ist die einzige Goroutine, die Reports speichert und den
// Fortschritt fortschreibt — seriell, wie von IMPLEMENTIERUNG.md
// Abschnitt 7.3 gefordert ("SQLite mag keine konkurrierenden Schreiber").
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
			result.Errors = append(result.Errors, fmt.Errorf("report konnte nicht gespeichert werden: %w", err))
			continue
		}
		if imported {
			result.New++
		} else {
			result.Skipped++
		}
	}
}

// advanceProgress vermerkt uid als abgeschlossen und persistiert state,
// falls sich die lückenlose Fortschrittsgrenze dadurch verändert hat.
func (uc *UseCase) advanceProgress(ctx context.Context, tracker *progressTracker, state *domainsync.State, uid uint32) error {
	lastUID, advanced := tracker.markDone(uid)
	if !advanced {
		return nil
	}

	state.LastUID = lastUID
	state.LastSyncAt = time.Now()
	if err := uc.States.Save(ctx, *state); err != nil {
		// Fortschritt konnte nicht gesichert werden — im nächsten Lauf
		// werden einzelne Nachrichten höchstens erneut verarbeitet, der
		// UNIQUE-Index fängt dabei entstehende Duplikate ab
		// (IMPLEMENTIERUNG.md Abschnitt 7.2). Kein Abbruch des Laufs
		// deswegen, aber im Result sichtbar (siehe write()).
		return fmt.Errorf("sync-fortschritt konnte nicht gespeichert werden: %w", err)
	}
	return nil
}
