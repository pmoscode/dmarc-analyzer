// Package syncjob orchestriert einen Sync-Lauf über alle konfigurierten
// Konten als serverseitiger Hintergrund-Auftrag: höchstens ein Lauf
// gleichzeitig, Abbruch per Kontext, Fortschritt für beliebig viele
// Zuhörer (MIGRATIONSPLAN.md Erweiterung 9.2). Derselbe Ablauf wie zuvor
// internal/ui/app.go (shell.startSync): alle Konten nacheinander,
// Ergebnisse aufsummiert, ein gemeinsamer Abbruch — hier serverseitig,
// weil der Sync über die eine HTTP-Anfrage hinaus weiterlaufen muss, die
// ihn angestoßen hat (Seitenwechsel, mehrere Browser-Tabs,
// MIGRATIONSPLAN.md Abschnitt 8: "Sync läuft serverseitig weiter").
package syncjob

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// Syncer ist der Ausschnitt von syncreports.UseCase, den Runner braucht —
// als eigene Schnittstelle statt einer konkreten Struct-Abhängigkeit,
// damit Tests einen Fake statt eines vollständig verdrahteten UseCase
// (mit IMAP-/Decoder-/Parser-Fakes) einsetzen können.
type Syncer interface {
	SyncAccount(ctx context.Context, id account.AccountID, onProgress syncreports.OnProgress) (syncreports.Result, error)
}

var _ Syncer = (*syncreports.UseCase)(nil)

// ErrAlreadyRunning meldet Start(), wenn bereits ein Lauf aktiv ist.
var ErrAlreadyRunning = errors.New("es läuft bereits ein abgleich")

// Status ist der grobe Zustand eines Sync-Laufs.
type Status string

// Werte für Status.
const (
	StatusIdle      Status = "idle"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
	StatusFailed    Status = "failed"
)

// State ist eine Momentaufnahme des aktuellen bzw. letzten Sync-Laufs —
// wird an jeden Zuhörer (Subscribe) verteilt.
type State struct {
	Status Status

	TotalAccounts  int
	AccountIndex   int    // 1-basiert, 0 außerhalb eines Laufs
	CurrentAccount string // DisplayName des gerade laufenden Kontos, leer außerhalb eines Laufs

	// Progress ist der Zwischenstand des gerade laufenden Kontos — wird
	// bei jedem neuen Konto zurückgesetzt (kein kumulativer Zähler über
	// mehrere Konten hinweg, siehe Total dafür).
	Progress syncreports.Progress
	// Total ist erst gesetzt, sobald der gesamte Lauf beendet ist (Done/
	// Cancelled/Failed) — die Summe über alle abgeschlossenen Konten.
	Total syncreports.Result

	// Err ist gesetzt, wenn Status Failed ist (z. B. Kontenliste konnte
	// nicht geladen werden). Ein Fehler bei einem EINZELNEN Konto führt
	// nicht zu Failed — der Lauf macht mit den restlichen Konten weiter,
	// wie zuvor internal/ui/app.go (dort "firstErr" genannt) — der erste
	// Kontofehler landet hier trotzdem, zur Anzeige nach Laufende.
	Err error

	StartedAt time.Time
	EndedAt   time.Time
}

// Runner führt höchstens einen Sync-Lauf gleichzeitig aus.
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

// NewRunner erzeugt einen Runner. baseCtx bestimmt die maximale
// Lebensdauer eines Laufs (im Produktivbetrieb der Serverlebenszyklus,
// siehe cmd_web.go) — NICHT der Kontext der einzelnen HTTP-Anfrage, die
// den Lauf per Start() anstößt.
func NewRunner(baseCtx context.Context, accounts account.Repository, sync Syncer) *Runner {
	return &Runner{
		baseCtx:     baseCtx,
		accounts:    accounts,
		sync:        sync,
		state:       State{Status: StatusIdle},
		subscribers: make(map[chan State]struct{}),
	}
}

// Start stößt einen neuen Lauf über alle konfigurierten Konten an.
// ErrAlreadyRunning, wenn schon einer läuft.
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

// Cancel bricht einen laufenden Sync ab — kein Fehler, wenn keiner läuft.
func (r *Runner) Cancel() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Snapshot liefert den aktuellen Stand, ohne zu abonnieren.
func (r *Runner) Snapshot() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Subscribe liefert einen Kanal, der ab sofort jede Zustandsänderung
// erhält — beginnend mit dem aktuellen Stand, damit ein neu
// hinzukommender Zuhörer (z. B. ein zweiter Browser-Tab) nicht erst auf
// die nächste Änderung warten muss. Der Kanal ist gepuffert (Größe 1)
// und verwirft bei einem noch nicht gelesenen, veralteten Stand diesen
// zugunsten des neuesten — ein langsamer oder getrennter Zuhörer darf
// den Sync selbst nie ausbremsen.
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

// broadcastLocked verteilt r.state an alle Zuhörer — muss mit
// gehaltenem r.mu aufgerufen werden.
func (r *Runner) broadcastLocked() {
	for ch := range r.subscribers {
		select {
		case ch <- r.state:
		default:
			// Kanal ist voll: ältesten, noch nicht gelesenen Stand
			// verwerfen und den neuesten nachreichen, statt zu blockieren.
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
	// ctx.Err() zusätzlich zu "cancelled" prüfen: cancelled wird nur vor
	// dem NÄCHSTEN Konto gesetzt (Zeile oben) — bricht der Kontext genau
	// während des letzten (oder einzigen) Kontos ab, verlässt die
	// Schleife danach ganz regulär, ohne dass "cancelled" je gesetzt
	// wurde. Ohne diese zweite Prüfung würde ein mitten im einzigen Konto
	// abgebrochener Lauf fälschlich als "done" gemeldet.
	if cancelled || ctx.Err() != nil {
		status = StatusCancelled
		// Ein durch Cancel() ausgelöster ctx.Canceled/DeadlineExceeded ist
		// die ERWARTETE Ursache für den Abbruch, kein eigenständiger
		// Fehler — sonst zeigte die Oberfläche "abgebrochen" UND einen
		// Fehlertext gleichzeitig für denselben, gewollten Vorgang an.
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
