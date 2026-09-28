package account

import (
	"context"
)

// Repository is the port for persisting MailAccount metadata. Implemented
// against SQLite (internal/infra/sqlite). At runtime there is exactly one
// account, loaded from ENV and upserted at startup (see
// internal/infra/envconfig, cmd/dmarc-analyzer/wire.go) — Repository stays
// generic nonetheless (FindAll instead of a single value) so that
// sync_state/reports/failed_imports keep their existing foreign keys to
// accounts(id) unchanged.
type Repository interface {
	Save(ctx context.Context, a *MailAccount) error
	FindByID(ctx context.Context, id AccountID) (*MailAccount, error)
	FindAll(ctx context.Context) ([]MailAccount, error)
}
