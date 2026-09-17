// Package retention setzt die Aufbewahrungsrichtlinie um (AP 7,
// IMPLEMENTIERUNG.md O-7: "Standard-Aufbewahrungsdauer für Reports? 24
// Monate, in den Einstellungen änderbar.") — die Dauer kommt seit dem
// Umstieg auf reine ENV-Konfiguration aus internal/infra/envconfig, nicht
// mehr aus einer zur Laufzeit änderbaren Einstellung.
package retention

import (
	"context"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// UseCase wendet die Aufbewahrungsrichtlinie an. Reports ist bewusst vom
// Typ report.Pruner statt report.Repository — dieser Anwendungsfall
// braucht nur DeleteOlderThan, keinen vollständigen Repository-Zugriff
// (siehe Kommentar an report.Pruner).
type UseCase struct {
	// RetentionMonths kommt aus envconfig.Config.RetentionMonths — 0
	// bedeutet unbegrenzte Aufbewahrung, keine automatische Löschung.
	RetentionMonths int
	Reports         report.Pruner

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

// ApplyNow löscht alle Reports, die nach der konfigurierten
// Aufbewahrungsdauer als zu alt gelten, und liefert die Anzahl gelöschter
// Reports. RetentionMonths <= 0 (unbegrenzte Aufbewahrung) löscht nichts.
func (u *UseCase) ApplyNow(ctx context.Context) (int64, error) {
	if u.RetentionMonths <= 0 {
		return 0, nil
	}

	cutoff := u.now().AddDate(0, -u.RetentionMonths, 0)
	return u.Reports.DeleteOlderThan(ctx, cutoff)
}
