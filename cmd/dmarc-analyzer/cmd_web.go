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

// retentionCheckInterval ist der Abstand zwischen zwei Anwendungen der
// Aufbewahrungsrichtlinie (AP 7) — Löschen alter Reports ist unauffällige
// Wartung, ein Tag Verzögerung nach einer geänderten Einstellung ist
// unproblematisch.
const retentionCheckInterval = 24 * time.Hour

// runWeb startet die eingebettete Web-Oberfläche. Lebenszyklus über
// SIGINT/SIGTERM — "docker stop" sendet SIGTERM, kein Auto-Ende, kein
// Beenden-Knopf: der Prozess verhält sich wie ein gewöhnlicher
// Container-Hauptprozess.
func runWeb(ctx context.Context, a *app, _ []string) error {
	// Eigener, auf diesen Aufruf begrenzter signal-Kontext statt den
	// übergebenen ctx global umzustellen — sync/import/stats sollen von
	// dieser Änderung im Lebenszyklus unberührt bleiben (AGENTS.md:
	// Composition Root verdrahtet nur, was der jeweilige Unterbefehl
	// tatsächlich braucht). Derselbe Kontext ist gleich unten die Basis
	// für syncjob.Runner und die Hintergrund-Aufträge.
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	syncJob := syncjob.NewRunner(signalCtx, a.accounts.Accounts, a.sync)

	srv, err := web.New(signalCtx, web.Dependencies{
		Statistics:          a.stats,
		Reports:             a.queries,
		Sources:             a.sourceStats,
		Domains:             a.domainStats,
		Accounts:            a.accounts,
		SyncJob:             syncJob,
		Importer:            a.importer,
		Retention:           a.retention,
		SyncIntervalMinutes: a.config.SyncIntervalMinutes,
		IMAPHost:            a.config.IMAPHost,
	}, web.Options{
		Addr: a.config.ListenAddr,
		Dev:  a.config.DevMode,
		OIDC: web.OIDCConfig{
			IssuerURL:    a.config.OIDC.IssuerURL,
			ClientID:     a.config.OIDC.ClientID,
			ClientSecret: a.config.OIDC.ClientSecret,
			RedirectURL:  a.config.OIDC.RedirectURL,
			AdminGroup:   a.config.OIDC.AdminGroup,
		},
	})
	if err != nil {
		return fmt.Errorf("web-oberfläche konnte nicht aufgebaut werden: %w", err)
	}

	// Hintergrund-Aufträge (AP 7) — laufen über die Lebensdauer des
	// Servers (signalCtx), unabhängig von einzelnen HTTP-Anfragen: die
	// Aufbewahrungsrichtlinie und der geplante Abgleich laufen auch, wenn
	// gerade niemand angemeldet ist.
	go retentionjob.NewRunner(a.retention, retentionCheckInterval, slog.Default()).Run(signalCtx)
	go syncscheduler.NewScheduler(syncJob, time.Duration(a.config.SyncIntervalMinutes)*time.Minute, slog.Default()).Run(signalCtx)

	if err := srv.Start(signalCtx); err != nil {
		return fmt.Errorf("web-oberfläche konnte nicht gestartet werden: %w", err)
	}

	fmt.Printf("dmarc-analyzer läuft: http://%s\n", srv.Addr())
	fmt.Println("Strg+C zum Beenden.")
	slog.Info("web-oberfläche gestartet", "addr", srv.Addr())

	<-signalCtx.Done()
	fmt.Println("\nWird beendet …")
	slog.Info("web-oberfläche wird beendet")

	return srv.Shutdown(context.Background())
}
