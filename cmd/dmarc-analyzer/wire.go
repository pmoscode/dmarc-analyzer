package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/domainoverview"
	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryfailedrecords"
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

// primaryAccountID is the fixed ID of the single, ENV-configured account
// (see internal/infra/envconfig) — since the move to pure ENV
// configuration there is no more account CRUD, only exactly one account
// per container.
const primaryAccountID = account.AccountID("primary")

// app bundles the use cases wired for the CLI and the loaded
// configuration.
type app struct {
	db     *sql.DB
	config envconfig.Config

	accounts      *manageaccount.UseCase
	queries       *queryreports.UseCase
	sync          *syncreports.UseCase
	importer      *importfiles.UseCase
	stats         *statistics.UseCase
	sourceStats   *sourcestats.UseCase
	domainStats   *domainoverview.UseCase
	failedRecords *queryfailedrecords.UseCase
	retention     *retention.UseCase
}

// newApp loads the ENV configuration, opens the database, creates the
// one configured account (or updates it if host/port/username have
// changed since the last start), and wires the use cases. The only
// place in the program that knows concrete infra types (AGENTS.md:
// "cmd/dmarc-analyzer: the only place where adapters are wired to use
// cases").
func newApp(ctx context.Context) (*app, error) {
	cfg, err := envconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("configuration could not be loaded: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "dmarc.db")
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("database could not be opened: %w", err)
	}

	accountRepo := sqlite.NewAccountRepository(db)
	reportRepo := sqlite.NewReportRepository(db)
	stateRepo := sqlite.NewSyncStateRepository(db)
	failedRepo := sqlite.NewFailedImportRepository(db)
	statsRepo := sqlite.NewStatisticsRepository(db)
	sourceStatsRepo := sqlite.NewSourceStatsRepository(db)
	domainStatsRepo := sqlite.NewDomainStatsRepository(db)
	failedRecordsRepo := sqlite.NewFailedRecordsRepository(db)

	acc, err := account.NewMailAccount(
		primaryAccountID, "", cfg.IMAPHost, cfg.IMAPPort, cfg.IMAPUser, cfg.IMAPMailbox, cfg.IMAPTLS, time.Now(),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("account from ENV configuration is invalid: %w", err)
	}
	if err := accountRepo.Save(ctx, acc); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("account could not be saved: %w", err)
	}

	decoder := mailmime.NewDecoder()
	parsers := []domainsync.ReportParser{dmarcxml.NewParser()}
	newSource := func() domainsync.MessageSource { return imap.NewAdapter() }
	// enricher: a shared instance for the dashboard and the sending
	// sources view — both enrich the same source IPs, a shared cache
	// avoids duplicate PTR lookups.
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
		domainStats: &domainoverview.UseCase{
			Domains: domainStatsRepo,
		},
		failedRecords: &queryfailedrecords.UseCase{
			Records: failedRecordsRepo,
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
