package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pmoscode/dmarc-analyzer/internal/app/retentionjob"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncscheduler"
	"github.com/pmoscode/dmarc-analyzer/internal/web"
)

// retentionCheckInterval is the interval between two applications of the
// retention policy (AP 7) — deleting old reports is unobtrusive
// maintenance, a one-day delay after a changed setting is unproblematic.
const retentionCheckInterval = 24 * time.Hour

// runWeb starts the embedded web UI. Lifecycle via SIGINT/SIGTERM —
// "docker stop" sends SIGTERM, there's no auto-shutdown and no shutdown
// button: the process behaves like an ordinary container main process.
func runWeb(ctx context.Context, a *app, _ []string) error {
	// A dedicated signal context scoped to this call, instead of globally
	// replacing the passed-in ctx — sync/import/stats should stay
	// unaffected by this lifecycle change (AGENTS.md: the Composition
	// Root only wires what the given subcommand actually needs). The
	// same context is also the basis below for syncjob.Runner and the
	// background jobs.
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	syncJob := syncjob.NewRunner(signalCtx, a.accounts.Accounts, a.sync)

	srv, err := web.New(signalCtx, web.Dependencies{
		Statistics:          a.stats,
		Reports:             a.queries,
		Sources:             a.sourceStats,
		Domains:             a.domainStats,
		FailedRecords:       a.failedRecords,
		Accounts:            a.accounts,
		SyncJob:             syncJob,
		Importer:            a.importer,
		Retention:           a.retention,
		SyncIntervalMinutes: a.config.SyncIntervalMinutes,
		IMAPHost:            a.config.IMAPHost,
	}, web.Options{
		Addr:  a.config.ListenAddr,
		Dev:   a.config.DevMode,
		Build: web.BuildInfo{Version: version, Commit: resolvedCommit()},
		OIDC: web.OIDCConfig{
			IssuerURL:          a.config.OIDC.IssuerURL,
			ClientID:           a.config.OIDC.ClientID,
			ClientSecret:       a.config.OIDC.ClientSecret,
			RedirectURL:        a.config.OIDC.RedirectURL,
			AdminGroup:         a.config.OIDC.AdminGroup,
			InsecureSkipVerify: a.config.OIDC.InsecureSkipVerify,
		},
	})
	if err != nil {
		return fmt.Errorf("web UI could not be built: %w", err)
	}

	// Background jobs (AP 7) — run for the lifetime of the server
	// (signalCtx), independent of individual HTTP requests: the
	// retention policy and the scheduled sync keep running even when
	// nobody is currently logged in.
	go retentionjob.NewRunner(a.retention, retentionCheckInterval, slog.Default()).Run(signalCtx)
	go syncscheduler.NewScheduler(syncJob, time.Duration(a.config.SyncIntervalMinutes)*time.Minute, slog.Default()).Run(signalCtx)

	if err := srv.Start(signalCtx); err != nil {
		return fmt.Errorf("web UI could not be started: %w", err)
	}

	fmt.Printf("dmarc-analyzer running: http://%s\n", srv.Addr())
	fmt.Println("Press Ctrl+C to stop.")
	slog.Info("web UI started", "addr", srv.Addr())

	<-signalCtx.Done()
	fmt.Println("\nShutting down …")
	slog.Info("web UI shutting down")

	return srv.Shutdown(context.Background())
}
