// Package syncscheduler stößt automatische Hintergrund-Abgleiche nach
// Zeitplan an (AP 7: "Hintergrund-Sync nach Zeitplan") — der manuell per
// /abgleich gestartete Lauf (internal/app/syncjob) bleibt davon unberührt
// und weiterhin jederzeit möglich.
package syncscheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

// Starter ist der Ausschnitt von syncjob.Runner, den Scheduler braucht.
// ErrAlreadyRunning wird nicht gesondert behandelt — passiert es, tut das
// gestartete Signal ohnehin nichts (ein Lauf läuft ja schon), also reicht
// Starter.Start()s eigenes "kein Fehler, wenn schon einer läuft" hier
// nicht: Scheduler ignoriert jeden Fehler von Start() bewusst, siehe tick().
type Starter interface {
	Start() error
	Snapshot() Snapshot
}

// Snapshot ist der für den Scheduler relevante Ausschnitt von
// syncjob.State — ein eigener, kleiner Typ statt einer Abhängigkeit auf
// syncjob, damit dieses Paket unabhängig von dessen internem Status-Enum
// bleibt (nur "läuft gerade" und "wann zuletzt beendet" zählen hier).
type Snapshot struct {
	Running bool
	// LastActivity ist der Start- oder Endzeitpunkt des letzten Laufs,
	// je nachdem was zuletzt gesetzt wurde — Zero, wenn seit dem Start des
	// Programms noch nie synchronisiert wurde.
	LastActivity time.Time
}

// Scheduler prüft in festen, kurzen Abständen (checkInterval), ob laut der
// aktuell gespeicherten Einstellungen ein automatischer Abgleich fällig
// ist. Die eigentliche Sync-Häufigkeit (settings.SyncIntervalMinutes) kann
// sich jederzeit ändern (Einstellungen-Seite) — ein fester
// time.Ticker(interval) würde das erst nach einem Neustart bemerken,
// dieses Nachfragen bei jedem Tick dagegen sofort.
type Scheduler struct {
	settings      settings.Repository
	job           Starter
	checkInterval time.Duration
	logger        *slog.Logger

	// now ist austauschbar für Tests mit einem festen Zeitpunkt.
	now func() time.Time
}

// NewScheduler erzeugt einen Scheduler. logger == nil verwendet
// slog.Default().
func NewScheduler(settingsRepo settings.Repository, job Starter, checkInterval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{settings: settingsRepo, job: job, checkInterval: checkInterval, logger: logger, now: time.Now}
}

// Run blockiert, bis ctx endet.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	cfg, err := s.settings.Load(ctx)
	if err != nil {
		s.logger.Error("einstellungen für geplanten abgleich konnten nicht gelesen werden", "error", err)
		return
	}
	if cfg.SyncIntervalMinutes <= 0 {
		return
	}

	snap := s.job.Snapshot()
	if snap.Running {
		return
	}
	if !snap.LastActivity.IsZero() && s.now().Sub(snap.LastActivity) < time.Duration(cfg.SyncIntervalMinutes)*time.Minute {
		return
	}

	if err := s.job.Start(); err != nil {
		// ErrAlreadyRunning ist harmlos (Race mit einem gerade manuell
		// gestarteten Lauf) — jeder andere Fehler (z. B. Kontenliste nicht
		// ladbar) landet ohnehin schon in syncjob.State.Err und wird dem
		// Nutzer dort angezeigt, hier reicht ein Log-Eintrag.
		s.logger.Warn("geplanter abgleich konnte nicht gestartet werden", "error", err)
	}
}
