package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is a single, numbered schema change.
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies all not-yet-applied migrations from migrations/*.sql in
// order, each within its own transaction. A small home-grown migrator
// instead of an extra dependency (IMPLEMENTIERUNG.md section 8.2).
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("migration %04d_%s failed: %w", m.version, m.name, err)
		}
	}

	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	const stmt = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("could not create schema_migrations: %w", err)
	}
	return nil
}

// loadMigrations reads all *.sql files from migrations/ and sorts them by
// their leading version number (filename "0001_init.sql" → 1).
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("could not read migrations directory: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, name, err := parseMigrationFilename(entry.Name())
		if err != nil {
			return nil, err
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("could not read migration %q: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{version: version, name: name, sql: string(content)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

// parseMigrationFilename expects the format "<four-digit-number>_<name>.sql".
func parseMigrationFilename(filename string) (version int, name string, err error) {
	base := strings.TrimSuffix(filename, ".sql")
	prefix, rest, found := strings.Cut(base, "_")
	if !found {
		return 0, "", fmt.Errorf("migration file %q does not follow the <number>_<name>.sql scheme", filename)
	}

	version, err = strconv.Atoi(prefix)
	if err != nil {
		return 0, "", fmt.Errorf("migration file %q does not have a valid version number: %w", filename, err)
	}

	return version, rest, nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("could not read applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("could not read migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not fully read migrations: %w", err)
	}

	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op if already committed

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("could not execute sql: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		m.version, m.name,
	); err != nil {
		return fmt.Errorf("could not record migration as applied: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	return nil
}
