// Package manageaccount provides diagnostic functions for the single
// mail account configured via ENV (see internal/infra/envconfig) —
// creating/deleting accounts no longer exists since the switch to ENV
// configuration, only "test connection" and "show account" remain.
package manageaccount

import (
	"context"
	"fmt"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// UseCase orchestrates diagnostics for the configured account.
type UseCase struct {
	Accounts account.Repository
	// Secret is the IMAP password loaded from ENV — valid for the entire
	// process lifetime, there is no longer any changing/saving at
	// runtime.
	Secret account.Secret
	// NewSource returns a fresh, unconnected MessageSource for each
	// connection test.
	NewSource func() sync.MessageSource
}

// TestConnectionByID loads the configured account and tests the
// connection — for the "test connection" button on the status page.
func (uc *UseCase) TestConnectionByID(ctx context.Context, id account.AccountID) error {
	acc, err := uc.Accounts.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("account %q could not be loaded: %w", id, err)
	}

	source := uc.NewSource()
	defer func() { _ = source.Close() }()

	if err := source.Connect(ctx, *acc, uc.Secret); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

// List returns all configured accounts — currently always exactly one
// (see internal/infra/envconfig), for the status page.
func (uc *UseCase) List(ctx context.Context) ([]account.MailAccount, error) {
	accounts, err := uc.Accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("accounts could not be loaded: %w", err)
	}
	return accounts, nil
}
