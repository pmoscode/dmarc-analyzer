// Package sqlite implements the repository ports from internal/domain
// against SQLite (modernc.org/sqlite, CGO-free — see IMPLEMENTIERUNG.md
// section 3 and 8).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver with database/sql
)

// Open opens (creating if necessary) the SQLite database at path and sets
// the PRAGMAs defined in IMPLEMENTIERUNG.md section 8.2 directly in the
// connection DSN: journal_mode=WAL, foreign_keys=ON, busy_timeout=5000,
// synchronous=NORMAL. Afterwards applies the migrations (see migrate.go).
//
// path is carried into the DSN unchanged (no URL escaping) — that's what
// modernc.org/sqlite expects (see its dsn_test.go). A path containing "?"
// or "#" would confuse the query-parameter boundary; for the path derived
// from DMARC_DATA_DIR (see internal/infra/envconfig) that practically
// never happens.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path +
		"?_journal_mode=WAL" +
		"&_foreign_keys=on" +
		"&_busy_timeout=5000" +
		"&_synchronous=NORMAL"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open database %q: %w", path, err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database %q is not responding: %w", path, err)
	}

	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("could not apply migrations: %w", err)
	}

	return db, nil
}
