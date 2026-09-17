package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncreports"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
	domainsync "github.com/pmoscode/dmarc-analyzer/internal/domain/sync"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/dmarcxml"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/envconfig"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/imap"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/mailmime"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sourceinfo"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/sqlite"
)

// primaryAccountID ist die feste ID des einen, per ENV konfigurierten
// Kontos (siehe internal/infra/envconfig) — es gibt seit dem Umstieg auf
// reine ENV-Konfiguration kein Konto-CRUD mehr, nur genau ein Konto pro
// Container.
const primaryAccountID = account.AccountID("primary")

// app bündelt die für die CLI verdrahteten Use Cases und die geladene
// Konfiguration.
type app struct {
	db     *sql.DB
	config envconfig.Config

	accounts    *manageaccount.UseCase
	queries     *queryreports.UseCase
	sync        *syncreports.UseCase
	importer    *importfiles.UseCase
	stats       *statistics.UseCase
	sourceStats *sourcestats.UseCase
	retention   *retention.UseCase
}

// newApp lädt die ENV-Konfiguration, öffnet die Datenbank, legt das eine
// konfigurierte Konto an (bzw. aktualisiert es, falls sich Host/Port/
// Benutzername seit dem letzten Start geändert haben) und verdrahtet die
// Use Cases. Einziger Ort im Programm, der konkrete Infra-Typen kennt
// (AGENTS.md: "cmd/dmarc-analyzer: einziger Ort, an dem Adapter mit Use
// Cases verdrahtet werden").
func newApp(ctx context.Context) (*app, error) {
	cfg, err := envconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("konfiguration konnte nicht geladen werden: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "dmarc.db")
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("datenbank konnte nicht geöffnet werden: %w", err)
	}

	accountRepo := sqlite.NewAccountRepository(db)
	reportRepo := sqlite.NewReportRepository(db)
	stateRepo := sqlite.NewSyncStateRepository(db)
	failedRepo := sqlite.NewFailedImportRepository(db)
	statsRepo := sqlite.NewStatisticsRepository(db)
	sourceStatsRepo := sqlite.NewSourceStatsRepository(db)

	acc, err := account.NewMailAccount(
		primaryAccountID, "", cfg.IMAPHost, cfg.IMAPPort, cfg.IMAPUser, cfg.IMAPMailbox, cfg.IMAPTLS, time.Now(),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("konto aus ENV-Konfiguration ist ungültig: %w", err)
	}
	if err := accountRepo.Save(ctx, acc); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("konto konnte nicht gespeichert werden: %w", err)
	}

	decoder := mailmime.NewDecoder()
	parsers := []domainsync.ReportParser{dmarcxml.NewParser()}
	newSource := func() domainsync.MessageSource { return imap.NewAdapter() }
	// enricher: eine gemeinsame Instanz für Dashboard und Sendequellen-
	// Ansicht — beide reichern dieselben Quell-IPs an, ein gemeinsamer
	// Cache spart doppelte PTR-Lookups.
	enricher := sourceinfo.NewEnricher()

	return &app{
		db:      db,
		config:  cfg,
		queries: &queryreports.UseCase{Reports: reportRepo},
		importer: &importfiles.UseCase{
			Reports:       reportRepo,
			FailedImports: failedRepo,
			Decoder:       decoder,
			Parsers:       parsers,
		},
		stats: &statistics.UseCase{Repository: statsRepo, Enricher: enricher},
		sourceStats: &sourcestats.UseCase{
			Sources:  sourceStatsRepo,
			Enricher: enricher,
		},
		retention: &retention.UseCase{
			RetentionMonths: cfg.RetentionMonths,
			Reports:         reportRepo,
		},
		accounts: &manageaccount.UseCase{
			Accounts:  accountRepo,
			Secret:    cfg.IMAPSecret,
			NewSource: newSource,
		},
		sync: &syncreports.UseCase{
			Accounts:      accountRepo,
			Secret:        cfg.IMAPSecret,
			States:        stateRepo,
			Reports:       reportRepo,
			FailedImports: failedRepo,
			Decoder:       decoder,
			Parsers:       parsers,
			NewSource:     newSource,
		},
	}, nil
}

func (a *app) Close() error {
	return a.db.Close()
}
