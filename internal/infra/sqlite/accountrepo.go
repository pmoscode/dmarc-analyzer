package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// AccountRepository implements account.Repository against SQLite.
type AccountRepository struct {
	db *sql.DB
}

var _ account.Repository = (*AccountRepository)(nil)

// NewAccountRepository creates a ready-to-use repository. db must already
// be opened via Open() (and thus migrated).
func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Save creates a new account or updates it (UPSERT on the ID) — without
// credentials, which come from ENV (see internal/infra/envconfig) and are
// never persisted.
func (r *AccountRepository) Save(ctx context.Context, a *account.MailAccount) error {
	const stmt = `
		INSERT INTO accounts (id, display_name, host, port, username, mailbox, use_tls, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			display_name = excluded.display_name,
			host         = excluded.host,
			port         = excluded.port,
			username     = excluded.username,
			mailbox      = excluded.mailbox,
			use_tls      = excluded.use_tls`

	_, err := r.db.ExecContext(ctx, stmt,
		string(a.ID), a.DisplayName, a.Host, a.Port, a.Username, a.Mailbox,
		boolToInt(a.UseTLS), a.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("could not save account %q: %w", a.ID, err)
	}
	return nil
}

// FindByID loads an account. Returns sql.ErrNoRows (wrapped) if none
// exists.
func (r *AccountRepository) FindByID(ctx context.Context, id account.AccountID) (*account.MailAccount, error) {
	const stmt = `
		SELECT id, display_name, host, port, username, mailbox, use_tls, created_at
		FROM accounts WHERE id = ?`

	row := r.db.QueryRowContext(ctx, stmt, string(id))
	acc, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("account %q: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("account %q konnte nicht geladen werden: %w", id, err)
	}
	return acc, nil
}

// FindAll loads all accounts, sorted by display name — for the account
// overview in settings (AP 5).
func (r *AccountRepository) FindAll(ctx context.Context) ([]account.MailAccount, error) {
	const stmt = `
		SELECT id, display_name, host, port, username, mailbox, use_tls, created_at
		FROM accounts ORDER BY display_name`

	rows, err := r.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("accounts konnten nicht geladen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var accounts []account.MailAccount
	for rows.Next() {
		acc, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("account-zeile konnte nicht gelesen werden: %w", err)
		}
		accounts = append(accounts, *acc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not fully read accounts: %w", err)
	}
	return accounts, nil
}

// rowScanner combines *sql.Row and *sql.Rows under a shared Scan
// signature, so FindByID and FindAll can share the same row mapping.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (*account.MailAccount, error) {
	var (
		id, displayName, host, username, mailbox, createdAt string
		port                                                int
		useTLS                                              int
	)

	if err := row.Scan(&id, &displayName, &host, &port, &username, &mailbox, &useTLS, &createdAt); err != nil {
		return nil, err
	}

	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("stored created_at is invalid: %w", err)
	}

	acc, err := account.NewMailAccount(
		account.AccountID(id), displayName, host, port, username, mailbox, useTLS != 0, created,
	)
	if err != nil {
		return nil, fmt.Errorf("stored account is invalid: %w", err)
	}
	return acc, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
