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

// version wird beim Release-Build per -ldflags gesetzt (siehe Taskfile.yml,
// Task "build"). Für lokale Entwicklungsbuilds bleibt "dev".
var version = "dev"

const usage = `dmarc-analyzer ` + `%s` + `

Ohne Argumente startet das grafische Programm. Für den Kommandozeilen-
Betrieb (z. B. Cron/launchd) stehen folgende Unterbefehle bereit:

Verwendung:
  dmarc-analyzer web [--adresse H:P] [--kein-browser] [--entwicklung]
                                             Web-Oberfläche starten (siehe MIGRATIONSPLAN.md) — vorerst
                                             neben dem grafischen Programm, das ohne Argumente startet
  dmarc-analyzer sync [--headless]          Alle konfigurierten Konten synchronisieren
  dmarc-analyzer import <pfad>...           DMARC-Reports aus Dateien importieren (.eml, .xml, .xml.gz, .zip)
  dmarc-analyzer stats [--days N] [--domain D]
                                             Kennzahlen für die letzten N Tage ausgeben (Standard: 30)
  dmarc-analyzer account add                Konto anlegen (fragt Zugangsdaten interaktiv ab)
  dmarc-analyzer account list               Konfigurierte Konten auflisten
  dmarc-analyzer account test <id>          Verbindung zu einem Konto testen
  dmarc-analyzer account delete <id>        Konto entfernen (Metadaten und Zugangsdaten)
`

// main entscheidet zwischen grafischem und Kommandozeilen-Betrieb — bewusst
// hier und nicht in run(): run() wird von cmd_*_test.go direkt mit leeren
// Argumenten aufgerufen (TestRun_NoArgs_PrintsUsageWithoutError) und darf
// dabei nie ein echtes Fenster öffnen, siehe cmd_gui.go.
func main() {
	args := os.Args[1:]

	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Printf(usage, version)
		return
	}

	if len(args) == 0 {
		if err := runGUI(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	logger := logging.New()
	logger.Info("dmarc-analyzer gestartet", "version", version)

	if len(args) == 0 {
		fmt.Printf(usage, version)
		return nil
	}

	cmd, rest := args[0], args[1:]

	handler, ok := subcommands[cmd]
	if !ok {
		return fmt.Errorf("unbekannter Unterbefehl %q — siehe 'dmarc-analyzer' ohne Argumente für die Hilfe", cmd)
	}

	// Composition Root: erst hier, nachdem der Unterbefehl als bekannt
	// erkannt wurde, werden konkrete Adapter verdrahtet (echte Datenbank,
	// echter OS-Schlüsselbund). Ein unbekannter Unterbefehl oder die reine
	// Hilfeausgabe lösen keinerlei I/O aus — wichtig sowohl für Tests als
	// auch dafür, dass ein Tippfehler nicht versehentlich die Datenbank
	// anlegt oder den Schlüsselbund anspricht.
	application, err := newApp(ctx, cmd)
	if err != nil {
		return fmt.Errorf("anwendung konnte nicht initialisiert werden: %w", err)
	}
	defer func() { _ = application.Close() }()

	return handler(ctx, application, rest)
}

// subcommands bildet Unterbefehlsnamen auf ihre Handler ab.
var subcommands = map[string]func(context.Context, *app, []string) error{
	"web":     runWeb,
	"sync":    runSync,
	"import":  runImport,
	"stats":   runStats,
	"account": runAccount,
}
