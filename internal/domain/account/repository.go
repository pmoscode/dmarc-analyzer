package account

import (
	"context"
)

// Repository ist der Port zur Persistenz der MailAccount-Metadaten.
// Implementiert gegen SQLite (internal/infra/sqlite). Es gibt zur Laufzeit
// genau ein Konto, aus ENV geladen und beim Start upgeserted (siehe
// internal/infra/envconfig, cmd/dmarc-analyzer/wire.go) — Repository bleibt
// trotzdem generisch (FindAll statt eines Einzelwerts), damit
// sync_state/reports/failed_imports ihre bestehenden Fremdschlüssel auf
// accounts(id) unverändert behalten.
type Repository interface {
	Save(ctx context.Context, a *MailAccount) error
	FindByID(ctx context.Context, id AccountID) (*MailAccount, error)
	FindAll(ctx context.Context) ([]MailAccount, error)
}
