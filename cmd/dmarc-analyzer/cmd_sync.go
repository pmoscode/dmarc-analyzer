package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// runSync syncs the ENV-configured account — for diagnostics/maintenance
// via "docker exec", in addition to the automatic background sync
// (internal/app/syncscheduler, which only runs under "web").
func runSync(ctx context.Context, a *app, _ []string) error {
	accounts, err := a.accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("accounts could not be loaded: %w", err)
	}
	if len(accounts) == 0 {
		return errors.New("no account configured — check the DMARC_IMAP_* environment variables")
	}

	var runErrs []error
	for _, acc := range accounts {
		result, err := a.sync.SyncAccount(ctx, acc.ID, nil)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("account %q: %w", acc.DisplayName, err))
			continue
		}

		fmt.Printf("%s: %d new, %d skipped, %d failed\n",
			acc.DisplayName, result.New, result.Skipped, result.Failed)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  error: %v\n", e)
		}
	}

	return errors.Join(runErrs...)
}
