package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// runSync synchronisiert das per ENV konfigurierte Konto — für Diagnose/
// Wartung per "docker exec" zusätzlich zum automatischen Hintergrund-Sync
// (internal/app/syncscheduler, läuft nur unter "web").
func runSync(ctx context.Context, a *app, _ []string) error {
	accounts, err := a.accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("konten konnten nicht geladen werden: %w", err)
	}
	if len(accounts) == 0 {
		return errors.New("kein Konto konfiguriert — DMARC_IMAP_*-Umgebungsvariablen prüfen")
	}

	var runErrs []error
	for _, acc := range accounts {
		result, err := a.sync.SyncAccount(ctx, acc.ID, nil)
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
