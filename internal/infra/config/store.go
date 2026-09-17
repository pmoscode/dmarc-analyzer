package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/settings"
)

// Store implementiert settings.Repository gegen eine einzelne JSON-Datei.
// Ein programmweiter Datensatz (kein Konto-, kein Report-Bezug) rechtfertigt
// keine eigene SQLite-Tabelle mit Migration — die Datei liegt neben
// instance.json im selben Konfigurationsverzeichnis (paths.ConfigDir()).
type Store struct {
	path string

	// mu schützt gegen gleichzeitige Save-Aufrufe aus mehreren
	// HTTP-Anfragen — Load braucht keinen Schutz (os.ReadFile ist bereits
	// nebenläufigkeitssicher), nur Save serialisiert Lesen+Schreiben nicht
	// atomar auf Dateisystemebene.
	mu sync.Mutex
}

// NewStore erzeugt einen Store, der unter path liest/schreibt. Existiert
// die Datei noch nicht, liefert Load settings.Default() ohne Fehler — der
// erste Start einer Installation ist kein Fehlerfall.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// fileFormat ist die Rohform der JSON-Datei — eigener Typ statt
// settings.Settings direkt zu (de-)serialisieren, damit ein künftiges
// Feld im Domänentyp nicht versehentlich das gespeicherte JSON-Format
// mitbestimmt (dieselbe Trennung wie sseState in internal/web für SSE).
type fileFormat struct {
	RetentionMonths     int `json:"retentionMonths"`
	SyncIntervalMinutes int `json:"syncIntervalMinutes"`
}

var _ settings.Repository = (*Store)(nil)

// Load liefert settings.Default(), wenn noch keine Datei existiert.
func (s *Store) Load(context.Context) (settings.Settings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return settings.Default(), nil
	}
	if err != nil {
		return settings.Settings{}, fmt.Errorf("einstellungen konnten nicht gelesen werden: %w", err)
	}

	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return settings.Settings{}, fmt.Errorf("einstellungsdatei %q ist beschädigt: %w", s.path, err)
	}

	return settings.Settings{
		RetentionMonths:     f.RetentionMonths,
		SyncIntervalMinutes: f.SyncIntervalMinutes,
	}, nil
}

// Save überschreibt die gespeicherten Einstellungen vollständig.
func (s *Store) Save(_ context.Context, v settings.Settings) error {
	if err := v.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(fileFormat{
		RetentionMonths:     v.RetentionMonths,
		SyncIntervalMinutes: v.SyncIntervalMinutes,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("einstellungen konnten nicht kodiert werden: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("einstellungen konnten nicht gespeichert werden: %w", err)
	}
	return nil
}
