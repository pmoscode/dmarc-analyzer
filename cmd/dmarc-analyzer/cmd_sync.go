package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

// runSync synchronisiert alle konfigurierten Konten
// (UMSETZUNGSPLAN.md AP 4: "dmarc-analyzer sync --headless").
func runSync(ctx context.Context, a *app, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	// --headless ist aktuell ein no-op: die CLI kennt ohnehin keine UI.
	// Das Flag existiert schon jetzt, damit AP 5 (grafisches Programm mit
	// echtem --headless-Modus) keinen Aufrufer-Bruch verursacht.
	_ = fs.Bool("headless", false, "ohne grafische Oberfläche ausführen (derzeit immer der Fall)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	accounts, err := a.accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("konten konnten nicht geladen werden: %w", err)
	}
	if len(accounts) == 0 {
		return errors.New("kein Konto konfiguriert — zuerst 'dmarc-analyzer account add' ausführen")
	}

	var runErrs []error
	for _, acc := range accounts {
		result, err := a.sync.SyncAccount(ctx, acc.ID)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("konto %q: %w", acc.DisplayName, err))
			continue
		}

		fmt.Printf("%s: %d neu, %d übersprungen, %d fehlerhaft\n",
			acc.DisplayName, result.New, result.Skipped, result.Failed)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  Fehler: %v\n", e)
		}
	}

	return errors.Join(runErrs...)
}
