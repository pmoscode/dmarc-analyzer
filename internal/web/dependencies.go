package web

import (
	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

// Dependencies bündelt die Use Cases, die die Web-Oberfläche braucht.
type Dependencies struct {
	Statistics *statistics.UseCase
	Reports    *queryreports.UseCase
	Sources    *sourcestats.UseCase
	Accounts   *manageaccount.UseCase
	SyncJob    *syncjob.Runner
	// Importer braucht keine Zugangsdaten — Import aus hochgeladenen
	// Dateien funktioniert unabhängig vom konfigurierten IMAP-Konto.
	Importer *importfiles.UseCase
	// Retention verwaltet die Aufbewahrungsrichtlinie (AP 7, siehe
	// handlers_settings.go) — RetentionMonths wird dort nur angezeigt,
	// geändert wird es per ENV und Container-Neustart.
	Retention *retention.UseCase
	// SyncIntervalMinutes ist rein informativ für die Status-Seite (siehe
	// handlers_settings.go) — der tatsächliche geplante Abgleich läuft in
	// internal/app/syncscheduler, das dieselbe ENV-Einstellung bekommt.
	SyncIntervalMinutes int
}
