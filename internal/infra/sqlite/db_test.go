package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

func TestOpen_AppliesPragmas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "pragmas.db")
	db, err := sqlite.Open(ctx, path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tests := map[string]string{
		"journal_mode": "wal",
		"foreign_keys": "1",
		"busy_timeout": "5000",
		"synchronous":  "1", // NORMAL
	}

	for pragma, want := range tests {
		var got string
		require.NoError(t, db.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got))
		require.Equal(t, want, got, "PRAGMA %s", pragma)
	}
}

func TestOpen_AppliesMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "migrated.db")
	db, err := sqlite.Open(ctx, path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var name string
	err = db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'reports'`).Scan(&name)
	require.NoError(t, err, "tabelle 'reports' muss nach dem öffnen existieren")
	require.Equal(t, "reports", name)
}

func TestMigrate_IsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "idempotent.db")
	db, err := sqlite.Open(ctx, path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Migrate erneut aufrufen darf nicht scheitern und keine Tabellen
	// doppelt anlegen.
	require.NoError(t, sqlite.Migrate(ctx, db))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count))
	require.Equal(t, 1, count, "jede migration darf nur einmal vermerkt sein")
}
