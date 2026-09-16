package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
)

// SyncStateRepository implementiert sync.StateRepository gegen SQLite.
type SyncStateRepository struct {
	db *sql.DB
}

var _ sync.StateRepository = (*SyncStateRepository)(nil)

// NewSyncStateRepository erzeugt ein einsatzbereites Repository.
func NewSyncStateRepository(db *sql.DB) *SyncStateRepository {
	return &SyncStateRepository{db: db}
}

// Load liefert den gespeicherten Fortschritt, oder den Nullwert (ohne
// Fehler), wenn noch keiner existiert — ein erster Sync ist kein
// Fehlerfall (siehe Port-Dokumentation in domain/sync/state.go).
func (r *SyncStateRepository) Load(ctx context.Context, accountID account.AccountID, mailbox string) (sync.State, error) {
	const stmt = `
		SELECT uid_validity, last_uid, last_sync_at
		FROM sync_state WHERE account_id = ? AND mailbox = ?`

	var uidValidity, lastUID int64
	var lastSyncAt sql.NullString
	err := r.db.QueryRowContext(ctx, stmt, string(accountID), mailbox).Scan(&uidValidity, &lastUID, &lastSyncAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sync.State{AccountID: accountID, Mailbox: mailbox}, nil
	}
	if err != nil {
		return sync.State{}, fmt.Errorf("sync-state für konto %q, postfach %q konnte nicht geladen werden: %w", accountID, mailbox, err)
	}

	uidValidity32, err := toUint32(uidValidity)
	if err != nil {
		return sync.State{}, fmt.Errorf("gespeicherte uid_validity ist ungültig: %w", err)
	}
	lastUID32, err := toUint32(lastUID)
	if err != nil {
		return sync.State{}, fmt.Errorf("gespeicherte last_uid ist ungültig: %w", err)
	}

	state := sync.State{
		AccountID:   accountID,
		Mailbox:     mailbox,
		UIDValidity: uidValidity32,
		LastUID:     lastUID32,
	}
	if lastSyncAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, lastSyncAt.String)
		if err != nil {
			return sync.State{}, fmt.Errorf("gespeichertes last_sync_at ist ungültig: %w", err)
		}
		state.LastSyncAt = t
	}
	return state, nil
}

// Save schreibt den Fortschritt fort (UPSERT über account_id+mailbox).
// LastSyncAt wird, falls nicht gesetzt, auf jetzt (UTC) gesetzt — ein Save
// ohne Zeitstempel würde sonst fälschlich "nie synchronisiert" bedeuten.
func (r *SyncStateRepository) Save(ctx context.Context, state sync.State) error {
	lastSyncAt := state.LastSyncAt
	if lastSyncAt.IsZero() {
		lastSyncAt = time.Now()
	}

	const stmt = `
		INSERT INTO sync_state (account_id, mailbox, uid_validity, last_uid, last_sync_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (account_id, mailbox) DO UPDATE SET
			uid_validity = excluded.uid_validity,
			last_uid     = excluded.last_uid,
			last_sync_at = excluded.last_sync_at`

	_, err := r.db.ExecContext(ctx, stmt,
		string(state.AccountID), state.Mailbox, state.UIDValidity, state.LastUID,
		lastSyncAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("sync-state für konto %q, postfach %q konnte nicht gespeichert werden: %w", state.AccountID, state.Mailbox, err)
	}
	return nil
}
