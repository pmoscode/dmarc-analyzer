// Command dmarc-analyzer ist die Composition Root der Anwendung: hier und
// nur hier werden konkrete Adapter mit den Use Cases verdrahtet
// (siehe IMPLEMENTIERUNG.md Abschnitt 4.1).
package main

import (
	"fmt"
	"os"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/logging"
)

// version wird beim Release-Build per -ldflags gesetzt (siehe Taskfile.yml,
// Task "build"). Für lokale Entwicklungsbuilds bleibt "dev".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	logger := logging.New()
	logger.Info("dmarc-analyzer gestartet", "version", version)

	// AP 0 — Grundgerüst: Programm startet, loggt seine Version und beendet
	// sich sauber. UI-Start und CLI-Unterbefehle (sync/import/stats) folgen
	// in AP 4 und AP 5.
	if len(args) > 0 {
		return fmt.Errorf("unbekanntes Argument %q — Unterbefehle folgen in AP 4", args[0])
	}

	return nil
}
