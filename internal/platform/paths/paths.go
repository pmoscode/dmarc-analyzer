// Package paths löst die plattformkonformen Speicherorte für Datenbank,
// Konfiguration und Logs auf (siehe IMPLEMENTIERUNG.md Abschnitt 8.2:
// os.UserConfigDir()/dmarc-analyzer/...).
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// appDirName ist der Unterordnername unter dem OS-Konfigverzeichnis.
const appDirName = "dmarc-analyzer"

// ConfigDir liefert das Verzeichnis für Konfiguration und Datenbank
// (z. B. macOS: ~/Library/Application Support/dmarc-analyzer,
// Linux: ~/.config/dmarc-analyzer, Windows: %AppData%\dmarc-analyzer).
// Das Verzeichnis wird bei Bedarf angelegt.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("konfigverzeichnis konnte nicht ermittelt werden: %w", err)
	}
	return ensureDir(filepath.Join(base, appDirName))
}

// DatabasePath liefert den vollständigen Pfad zur SQLite-Datenbankdatei.
func DatabasePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dmarc.db"), nil
}

// LogDir liefert das Verzeichnis für Log-Dateien
// (z. B. macOS: ~/Library/Logs/dmarc-analyzer).
// Das Verzeichnis wird bei Bedarf angelegt.
func LogDir() (string, error) {
	base, err := userLogDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(base, appDirName))
}

// LogFilePath liefert den vollständigen Pfad zur persistenten Log-Datei
// (siehe LogDir()). Wichtig vor allem für den Windows-Release-Build ohne
// Konsolenfenster (MIGRATIONSPLAN.md M5: "-H windowsgui") — dort verpufft
// alles, was nur nach os.Stderr geschrieben wird, spurlos; siehe
// cmd/dmarc-analyzer/cmd_web.go.
func LogFilePath() (string, error) {
	dir, err := LogDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dmarc-analyzer.log"), nil
}

func ensureDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("verzeichnis %q konnte nicht angelegt werden: %w", dir, err)
	}
	return dir, nil
}
