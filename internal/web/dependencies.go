package web

import (
	"github.com/pmoscode/dmarc-analyzer/internal/app/domainoverview"
	"github.com/pmoscode/dmarc-analyzer/internal/app/importfiles"
	"github.com/pmoscode/dmarc-analyzer/internal/app/manageaccount"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryfailedrecords"
	"github.com/pmoscode/dmarc-analyzer/internal/app/queryreports"
	"github.com/pmoscode/dmarc-analyzer/internal/app/retention"
	"github.com/pmoscode/dmarc-analyzer/internal/app/sourcestats"
	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

// Dependencies bundles the use cases the web UI needs.
type Dependencies struct {
	Statistics *statistics.UseCase
	Reports    *queryreports.UseCase
	Sources    *sourcestats.UseCase
	Domains    *domainoverview.UseCase
	// FailedRecords powers the failures view (/fehlschlaege) — a
	// cross-report search for records where DMARC failed (a complement to
	// the report detail page, which shows the same raw-data breakdown for
	// a single report).
	FailedRecords *queryfailedrecords.UseCase
	Accounts      *manageaccount.UseCase
	SyncJob       *syncjob.Runner
	// Importer needs no credentials — importing from uploaded files works
	// independently of the configured IMAP account.
	Importer *importfiles.UseCase
	// Retention manages the retention policy (AP 7, see
	// handlers_settings.go) — RetentionMonths is only displayed there;
	// changing it happens via ENV and a container restart.
	Retention *retention.UseCase
	// SyncIntervalMinutes is purely informational for the status page
	// (see handlers_settings.go) — the actual scheduled sync runs in
	// internal/app/syncscheduler, which gets the same ENV setting.
	SyncIntervalMinutes int
	// IMAPHost is the hostname of the configured IMAP account (from
	// DMARC_IMAP_HOST) — purely informational for the sending-sources view
	// (see handlers_sources.go: does a sending source share a host with
	// the IMAP account?). Not a credential, safe to carry here.
	IMAPHost string
}
