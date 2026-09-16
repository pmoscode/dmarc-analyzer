package main

import (
	"context"
	"fmt"

	fyneapp "fyne.io/fyne/v2/app"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/charts"
	"github.com/pmoscode/dmarc-analyzer/internal/ui"
)

// runGUI startet das grafische Programm (UMSETZUNGSPLAN.md AP 5/6) — der
// Standardeinstieg, wenn main() ohne Unterbefehl aufgerufen wird (siehe
// main.go). Bewusst nicht über run()/subcommands verdrahtet: run() wird
// von cmd_*_test.go direkt mit leeren Argumenten aufgerufen (u. a.
// TestRun_NoArgs_PrintsUsageWithoutError) und darf dabei nie ein echtes
// Fenster öffnen — die GUI-Entscheidung liegt deshalb allein in main().
//
// window.ShowAndRun() blockiert, bis der Nutzer das Fenster schließt —
// erst danach schließt der deferred a.Close() die Datenbank.
func runGUI(ctx context.Context) error {
	a, err := newApp(ctx, "gui")
	if err != nil {
		return fmt.Errorf("anwendung konnte nicht initialisiert werden: %w", err)
	}
	defer func() { _ = a.Close() }()

	fyneApp := fyneapp.New()
	deps := ui.Dependencies{
		Accounts:   a.accounts,
		Queries:    a.queries,
		Sync:       a.sync,
		Statistics: a.stats,
		Sources:    a.sourceStats,
		Charts:     charts.NewRenderer(),
	}

	window := ui.BuildMainWindow(fyneApp, deps)
	window.ShowAndRun()
	return nil
}
