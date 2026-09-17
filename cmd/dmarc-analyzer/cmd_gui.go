package main

import (
	"context"

	fyneapp "fyne.io/fyne/v2/app"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/charts"
	"github.com/pmoscode/dmarc-analyzer/internal/ui"
)

// runGUI startet das grafische Programm (UMSETZUNGSPLAN.md AP 5/6) — bis
// M2 der Standardeinstieg ohne Unterbefehl, seit M3 nur noch über den
// versteckten Unterbefehl "gui" erreichbar (MIGRATIONSPLAN.md M3:
// "Umschalten: dmarc-analyzer ohne Argumente startet die
// Web-Oberfläche"). Bleibt bis zum Rückbau in M5 als manueller
// Vergleichs-/Rückfallpfad nutzbar, ist aber nirgends mehr dokumentiert
// beworben.
//
// window.ShowAndRun() blockiert, bis der Nutzer das Fenster schließt.
func runGUI(_ context.Context, a *app, _ []string) error {
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
