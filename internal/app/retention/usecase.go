// Package retention setzt die Aufbewahrungsrichtlinie um (AP 7,
// IMPLEMENTIERUNG.md O-7: "Standard-Aufbewahrungsdauer für Reports? 24
// Monate, in den Einstellungen änderbar."): Lesen/Ändern der
// Einstellungen sowie das tatsächliche Löschen zu alter Reports.
package retention

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

// UseCase bündelt Lesen/Ändern der Einstellungen und das Anwenden der
// Aufbewahrungsrichtlinie. Reports ist bewusst vom Typ report.Pruner statt
// report.Repository — dieser Anwendungsfall braucht nur DeleteOlderThan,
// keinen vollständigen Repository-Zugriff (siehe Kommentar an
// report.Pruner).
type UseCase struct {
	Settings settings.Repository
	Reports  report.Pruner

	// Now liefert den aktuellen Zeitpunkt — nil verwendet time.Now.
	// Austauschbar für Tests mit einem festen Zeitpunkt.
	Now func() time.Time
}

func (u *UseCase) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

// LoadSettings liefert die aktuell gültigen Einstellungen.
func (u *UseCase) LoadSettings(ctx context.Context) (settings.Settings, error) {
	return u.Settings.Load(ctx)
}

// SaveSettings validiert und speichert neue Einstellungen.
func (u *UseCase) SaveSettings(ctx context.Context, s settings.Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return u.Settings.Save(ctx, s)
}

// ApplyNow löscht alle Reports, die nach der aktuell gespeicherten
// Aufbewahrungsdauer als zu alt gelten, und liefert die Anzahl gelöschter
// Reports. RetentionMonths == 0 (unbegrenzte Aufbewahrung) löscht nichts.
func (u *UseCase) ApplyNow(ctx context.Context) (int64, error) {
	s, err := u.Settings.Load(ctx)
	if err != nil {
		return 0, err
	}
	if s.RetentionMonths <= 0 {
		return 0, nil
	}

	cutoff := u.now().AddDate(0, -s.RetentionMonths, 0)
	return u.Reports.DeleteOlderThan(ctx, cutoff)
}
