package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// userLogDir liefert unter macOS ~/Library/Logs.
func userLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home-verzeichnis konnte nicht ermittelt werden: %w", err)
	}
	return filepath.Join(home, "Library", "Logs"), nil
}
