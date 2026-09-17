package web

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser öffnet url im Standardbrowser — ohne zusätzliche
// Abhängigkeit über os/exec (MIGRATIONSPLAN.md Abschnitt 3). Schlägt das
// fehl (kein Browser, SSH-Sitzung, unbekanntes Betriebssystem), meldet es
// den Fehler zurück — der Aufrufer zeigt die Adresse dann stattdessen auf
// der Konsole an, statt abzubrechen.
func OpenBrowser(url string) error {
	// context.Background(): der Browser-Prozess wird bewusst losgelöst
	// gestartet (cmd.Start(), nicht Run()) und soll weiterlaufen, auch
	// wenn der Server später beendet wird — kein Kontext, dessen
	// Abbruch den Browser mitreißen dürfte.
	ctx := context.Background()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		//nolint:gosec // G204: url stammt aus server.go (eigener,
		// zufällig erzeugter Einmal-Anmeldelink auf 127.0.0.1), nie aus
		// Nutzer- oder Netz-Eingaben.
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		// rundll32 statt "cmd /c start", weil start ein & im Titel als
		// Befehlstrenner missversteht — rundll32 reicht die URL direkt an
		// den Standardhandler durch.
		//nolint:gosec // G204: siehe Begründung oben.
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		//nolint:gosec // G204: siehe Begründung oben.
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("browser konnte nicht geöffnet werden: %w", err)
	}
	return nil
}
