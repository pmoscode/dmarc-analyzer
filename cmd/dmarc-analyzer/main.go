// Command dmarc-analyzer ist die Composition Root der Anwendung: hier und
// nur hier werden konkrete Adapter mit den Use Cases verdrahtet
// (siehe IMPLEMENTIERUNG.md Abschnitt 4.1).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/logging"
)

// version und commit werden beim Build per -ldflags gesetzt (siehe
// Taskfile.yml Task "build", Dockerfile, .github/workflows/ci.yml). Für
// lokale Entwicklungsbuilds bleibt version "dev"; ein leerer commit wird
// zur Laufzeit aus den Go-Build-Informationen ergänzt (siehe version.go).
var (
	version = "dev"
	commit  = ""
)

const usage = `dmarc-analyzer ` + `%s` + `

Läuft als Docker-Container, komplett über Umgebungsvariablen konfiguriert
(siehe docs/features/deployment.md). Ohne Argumente startet die
Web-Oberfläche. Für Diagnose/Wartung per "docker exec" stehen folgende
Unterbefehle bereit:

Verwendung:
  dmarc-analyzer web                        Web-Oberfläche starten (auch ohne Argumente der Standard)
  dmarc-analyzer sync                       Konfiguriertes Konto synchronisieren
  dmarc-analyzer import <pfad>...           DMARC-Reports aus Dateien importieren (.eml, .xml, .xml.gz, .zip)
  dmarc-analyzer stats [--days N] [--domain D]
                                             Kennzahlen für die letzten N Tage ausgeben (Standard: 30)
  dmarc-analyzer healthcheck                 Docker-HEALTHCHECK: prüft, ob der HTTP-Server antwortet
`

// main entscheidet, ob main.go selbst ein Argument ergänzen muss (kein
// Unterbefehl → "web", MIGRATIONSPLAN.md M3), bevor run() den
// eigentlichen Unterbefehl verarbeitet.
func main() {
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Printf(usage, version)
		return
	}

	if len(args) == 0 {
		// Web-Oberfläche ist seit M3 der Standard ohne Argumente.
		args = []string{"web"}
	}

	if err := run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Printf(usage, version)
		return nil
	}

	cmd, rest := args[0], args[1:]

	// healthcheck läuft bewusst ohne Logger-Setup und ohne newApp()/
	// envconfig.Load() weiter unten — Docker ruft es alle paar Sekunden
	// auf, ein Log-Eintrag pro Aufruf wäre reines Rauschen, und eine
	// Liveness-Prüfung soll nicht an einer kurzzeitig ungültigen
	// IMAP-/OIDC-Konfiguration scheitern (siehe cmd_healthcheck.go).
	if cmd == "healthcheck" {
		return runHealthcheck(ctx, rest)
	}

	logger := logging.New()
	logger.Info("dmarc-analyzer gestartet", "version", version, "commit", resolvedCommit())

	handler, ok := subcommands[cmd]
	if !ok {
		return fmt.Errorf("unbekannter Unterbefehl %q — siehe 'dmarc-analyzer' ohne Argumente für die Hilfe", cmd)
	}

	// Composition Root: erst hier, nachdem der Unterbefehl als bekannt
	// erkannt wurde, werden konkrete Adapter verdrahtet (ENV-Konfiguration
	// gelesen, echte Datenbank geöffnet). Ein unbekannter Unterbefehl oder
	// die reine Hilfeausgabe lösen keinerlei I/O aus — wichtig sowohl für
	// Tests als auch dafür, dass ein Tippfehler nicht versehentlich die
	// Datenbank anlegt.
	application, err := newApp(ctx)
	if err != nil {
		return fmt.Errorf("anwendung konnte nicht initialisiert werden: %w", err)
	}
	defer func() { _ = application.Close() }()

	return handler(ctx, application, rest)
}

// subcommands bildet Unterbefehlsnamen auf ihre Handler ab.
var subcommands = map[string]func(context.Context, *app, []string) error{
	"web":    runWeb,
	"sync":   runSync,
	"import": runImport,
	"stats":  runStats,
}
