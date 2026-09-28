package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// FailedImportRepository implements sync.FailedImportRepository against
// SQLite (table failed_imports).
type FailedImportRepository struct {
	db *sql.DB
}

var _ sync.FailedImportRepository = (*FailedImportRepository)(nil)

// NewFailedImportRepository creates a ready-to-use repository.
func NewFailedImportRepository(db *sql.DB) *FailedImportRepository {
	return &FailedImportRepository{db: db}
}

// Record notes a failed import. Deliberately no foreign key to
// accounts(id) in the schema — a failed import should remain visible even
// after the account has since been deleted.
func (r *FailedImportRepository) Record(ctx context.Context, f sync.FailedImport) error {
	occurredAt := f.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	const stmt = `
		INSERT INTO failed_imports (account_id, message_uid, filename, error, raw, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, stmt,
		nullableString(string(f.AccountID)), nullableUint32(f.MessageUID), nullableString(f.Filename),
		f.Error, f.Raw, occurredAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("could not record failed import: %w", err)
	}
	return nil
}
