// Package settings enthält die nicht-geheimen, programmweiten
// Einstellungen (AP 7: Aufbewahrungsrichtlinie, Hintergrund-Sync-Intervall)
// sowie den Port, über den sie persistiert werden. Bewusst getrennt von
// account.MailAccount: dort stehen Kontodaten, hier programmweite
// Vorgaben, die unabhängig von einem einzelnen Konto gelten.
package settings

import (
	"context"
	"fmt"
)

// Vorgabewerte, falls noch keine Einstellungen gespeichert sind — siehe
// IMPLEMENTIERUNG.md O-7 ("Standard-Aufbewahrungsdauer für Reports? 24
// Monate, in den Einstellungen änderbar.") und UMSETZUNGSPLAN.md AP 7.
const (
	DefaultRetentionMonths     = 24
	DefaultSyncIntervalMinutes = 60
)

// Settings sind die aktuell gültigen, programmweiten Einstellungen.
type Settings struct {
	// RetentionMonths ist die Aufbewahrungsdauer für Reports in Monaten,
	// gerechnet ab dem Ende ihres Berichtszeitraums (DateRange.End). 0
	// bedeutet: unbegrenzt aufbewahren, keine automatische Löschung.
	RetentionMonths int
	// SyncIntervalMinutes ist der Abstand zwischen zwei automatischen
	// Hintergrund-Abgleichen in Minuten. 0 bedeutet: kein automatischer
	// Abgleich, nur der manuell angestoßene über /abgleich.
	SyncIntervalMinutes int
}

// Default liefert die Vorgabewerte für eine neue Installation.
func Default() Settings {
	return Settings{
		RetentionMonths:     DefaultRetentionMonths,
		SyncIntervalMinutes: DefaultSyncIntervalMinutes,
	}
}

// Validate prüft, ob s gespeichert werden darf — negative Werte sind
// fachlich nicht sinnvoll (0 hat die eigene Bedeutung "aus/unbegrenzt",
// siehe Feldkommentare).
func (s Settings) Validate() error {
	if s.RetentionMonths < 0 {
		return fmt.Errorf("aufbewahrungsdauer darf nicht negativ sein")
	}
	if s.SyncIntervalMinutes < 0 {
		return fmt.Errorf("sync-intervall darf nicht negativ sein")
	}
	return nil
}

// Repository ist der Port zur Persistenz von Settings — ein einzelner,
// programmweiter Datensatz statt einer Tabelle (implementiert in
// internal/infra/config gegen eine JSON-Datei im Konfigurationsverzeichnis,
// AP 7).
type Repository interface {
	// Load liefert die gespeicherten Einstellungen, oder Default(), wenn
	// noch keine gespeichert wurden — kein Fehlerfall, analog zu
	// sync.StateRepository.Load.
	Load(ctx context.Context) (Settings, error)
	Save(ctx context.Context, s Settings) error
}
