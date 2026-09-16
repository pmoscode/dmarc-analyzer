package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// FailedImportRepository implementiert sync.FailedImportRepository gegen
// SQLite (Tabelle failed_imports).
type FailedImportRepository struct {
	db *sql.DB
}

var _ sync.FailedImportRepository = (*FailedImportRepository)(nil)

// NewFailedImportRepository erzeugt ein einsatzbereites Repository.
func NewFailedImportRepository(db *sql.DB) *FailedImportRepository {
	return &FailedImportRepository{db: db}
}

// Record vermerkt einen fehlgeschlagenen Import. Absichtlich kein
// Fremdschlüssel auf accounts(id) im Schema — ein fehlerhafter Import soll
// auch dann einsehbar bleiben, wenn das Konto inzwischen gelöscht wurde.
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
		return fmt.Errorf("fehlgeschlagener import konnte nicht vermerkt werden: %w", err)
	}
	return nil
}
