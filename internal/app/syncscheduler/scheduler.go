// Package syncscheduler stößt automatische Hintergrund-Abgleiche nach
// einem festen Zeitplan an (AP 7: "Hintergrund-Sync nach Zeitplan") — der
// manuell per /abgleich gestartete Lauf (internal/app/syncjob) bleibt
// davon unberührt und weiterhin jederzeit möglich. Das Intervall kommt aus
// envconfig.Config.SyncIntervalMinutes und ändert sich nicht zur Laufzeit
// (12-factor, kein Nachladen nötig — anders als vor dem Umstieg auf reine
// ENV-Konfiguration, siehe Git-Historie dieser Datei).
package syncscheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

// Starter ist der Ausschnitt von syncjob.Runner, den Scheduler braucht.
type Starter interface {
	Start() error
}

// Scheduler löst nach jedem interval einen Lauf aus, solange keiner läuft
// — läuft bereits einer, meldet syncjob.Runner.Start() harmlos
// ErrAlreadyRunning, das Intervall lässt den nächsten Versuch dann
// automatisch etwas später greifen.
type Scheduler struct {
	job      Starter
	interval time.Duration
	logger   *slog.Logger
}

// NewScheduler erzeugt einen Scheduler. logger == nil verwendet
// slog.Default(). interval <= 0 bedeutet: automatischer Abgleich
// deaktiviert (Run() kehrt dann sofort zurück).
func NewScheduler(job Starter, interval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{job: job, interval: interval, logger: logger}
}

// Run blockiert, bis ctx endet — ein erster Lauf sofort beim Start (ein
// frisch gestarteter Container soll nicht erst ein volles Intervall auf
// den ersten Abgleich warten), danach alle interval.
func (s *Scheduler) Run(ctx context.Context) {
	if s.interval <= 0 {
		return
	}

	s.trigger()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.trigger()
		}
	}
}

func (s *Scheduler) trigger() {
	err := s.job.Start()
	if err == nil || errors.Is(err, syncjob.ErrAlreadyRunning) {
		return
	}
	s.logger.Warn("geplanter abgleich konnte nicht gestartet werden", "error", err)
}
