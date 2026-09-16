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

// migration ist eine einzelne, nummerierte Schemaänderung.
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate wendet alle noch nicht angewendeten Migrationen aus
// migrations/*.sql der Reihe nach an, innerhalb je einer eigenen
// Transaktion. Eigener, kleiner Migrator statt eines zusätzlichen
// Dependencys (IMPLEMENTIERUNG.md Abschnitt 8.2).
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
			return fmt.Errorf("migration %04d_%s fehlgeschlagen: %w", m.version, m.name, err)
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
		return fmt.Errorf("schema_migrations konnte nicht angelegt werden: %w", err)
	}
	return nil
}

// loadMigrations liest alle *.sql-Dateien aus migrations/ und sortiert sie
// nach ihrer führenden Versionsnummer (Dateiname "0001_init.sql" → 1).
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations-verzeichnis konnte nicht gelesen werden: %w", err)
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
			return nil, fmt.Errorf("migration %q konnte nicht gelesen werden: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{version: version, name: name, sql: string(content)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

// parseMigrationFilename erwartet das Format "<vierstellige-Nummer>_<name>.sql".
func parseMigrationFilename(filename string) (version int, name string, err error) {
	base := strings.TrimSuffix(filename, ".sql")
	prefix, rest, found := strings.Cut(base, "_")
	if !found {
		return 0, "", fmt.Errorf("migrationsdatei %q folgt nicht dem schema <nummer>_<name>.sql", filename)
	}

	version, err = strconv.Atoi(prefix)
	if err != nil {
		return 0, "", fmt.Errorf("migrationsdatei %q hat keine gültige versionsnummer: %w", filename, err)
	}

	return version, rest, nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("angewendete migrationen konnten nicht gelesen werden: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("migrationsversion konnte nicht gelesen werden: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrationen konnten nicht vollständig gelesen werden: %w", err)
	}

	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transaktion konnte nicht gestartet werden: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op, wenn bereits committed

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("sql konnte nicht ausgeführt werden: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		m.version, m.name,
	); err != nil {
		return fmt.Errorf("migration konnte nicht als angewendet vermerkt werden: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaktion konnte nicht committed werden: %w", err)
	}

	return nil
}
