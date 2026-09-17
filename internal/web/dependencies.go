package web

import (
	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// Dependencies bündelt die Use Cases, die die Web-Oberfläche braucht —
// analog zu internal/ui.Dependencies, das dieses Paket in M5 ersetzt.
type Dependencies struct {
	Statistics *statistics.UseCase
	Reports    *queryreports.UseCase
	Sources    *sourcestats.UseCase
	Accounts   *manageaccount.UseCase
	// Credentials ist derselbe Store, den Accounts/SyncJob intern schon
	// benutzen — die Web-Oberfläche braucht ihn zusätzlich direkt für
	// /entsperren (Locked()/Unlock(), siehe handlers_unlock.go und
	// MIGRATIONSPLAN.md Erweiterung 9.4). nil (OS-Schlüsselbund oder
	// Nicht-web-Aufrufer) bedeutet: nie gesperrt.
	Credentials account.CredentialStore
	SyncJob     *syncjob.Runner
	// Importer braucht (anders als Accounts) keine Zugangsdaten — Import
	// aus hochgeladenen Dateien funktioniert auch bei gesperrtem
	// Schlüsselspeicher (MIGRATIONSPLAN.md Meilenstein M4).
	Importer *importfiles.UseCase
	// Retention verwaltet Aufbewahrungsdauer und Sync-Intervall
	// (AP 7, siehe handlers_settings.go) — braucht ebenfalls keine
	// Zugangsdaten.
	Retention *retention.UseCase
}
