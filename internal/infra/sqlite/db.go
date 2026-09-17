// Package sqlite implementiert die Repository-Ports aus internal/domain
// gegen SQLite (modernc.org/sqlite, CGO-frei — siehe IMPLEMENTIERUNG.md
// Abschnitt 3 und 8).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registriert den Treiber "sqlite" bei database/sql
)

// Open öffnet (und legt bei Bedarf an) die SQLite-Datenbank unter path und
// setzt die in IMPLEMENTIERUNG.md Abschnitt 8.2 festgelegten PRAGMAs direkt
// in der Verbindungs-DSN: journal_mode=WAL, foreign_keys=ON,
// busy_timeout=5000, synchronous=NORMAL. Wendet anschließend die
// Migrationen an (siehe migrate.go).
//
// path wird unverändert in die DSN übernommen (kein URL-Escaping) — so
// erwartet es modernc.org/sqlite (siehe dessen dsn_test.go). Ein Pfad mit
// "?" oder "#" würde die Query-Parameter-Grenze verwirren; für den aus
// DMARC_DATA_DIR abgeleiteten Pfad (siehe internal/infra/envconfig)
// kommt das praktisch nicht vor.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path +
		"?_journal_mode=WAL" +
		"&_foreign_keys=on" +
		"&_busy_timeout=5000" +
		"&_synchronous=NORMAL"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("datenbank %q konnte nicht geöffnet werden: %w", path, err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("datenbank %q antwortet nicht: %w", path, err)
	}

	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrationen konnten nicht angewendet werden: %w", err)
	}

	return db, nil
}
