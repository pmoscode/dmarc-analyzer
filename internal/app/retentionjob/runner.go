// Package retentionjob wendet die Aufbewahrungsrichtlinie (AP 7, siehe
// internal/app/retention) automatisch im Hintergrund an — einmal beim
// Start und danach in festen Abständen, analog zu internal/app/syncjob,
// nur ohne Fortschrittsanzeige: das Löschen alter Reports ist eine
// unauffällige Wartungsaufgabe, kein Vorgang, den der Nutzer beobachten
// oder abbrechen können muss.
package retentionjob

import (
	"context"
	"log/slog"
	"time"
)

// Applier ist der Ausschnitt von retention.UseCase, den Runner braucht —
// eine eigene Schnittstelle statt einer konkreten Struct-Abhängigkeit,
// damit Tests einen Fake statt eines vollständig verdrahteten UseCase
// einsetzen können (dieselbe Idee wie syncjob.Syncer).
type Applier interface {
	ApplyNow(ctx context.Context) (int64, error)
}

// Runner ruft Applier.ApplyNow periodisch auf, bis ctx endet.
type Runner struct {
	applier  Applier
	interval time.Duration
	logger   *slog.Logger
}

// NewRunner erzeugt einen Runner. logger == nil verwendet slog.Default().
func NewRunner(applier Applier, interval time.Duration, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{applier: applier, interval: interval, logger: logger}
}

// Run blockiert, bis ctx endet — ein erster Durchlauf sofort beim Start
// (eine frisch geänderte Aufbewahrungsdauer soll nicht erst nach einem
// vollen Intervall wirken), danach alle interval.
func (r *Runner) Run(ctx context.Context) {
	r.apply(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.apply(ctx)
		}
	}
}

func (r *Runner) apply(ctx context.Context) {
	deleted, err := r.applier.ApplyNow(ctx)
	if err != nil {
		r.logger.Error("aufbewahrungsrichtlinie konnte nicht angewendet werden", "error", err)
		return
	}
	if deleted > 0 {
		r.logger.Info("aufbewahrungsrichtlinie angewendet", "geloeschte_reports", deleted)
	}
}
