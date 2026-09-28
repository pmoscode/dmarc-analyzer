# Architecture

`dmarc-analyzer` follows Clean Architecture: four layers, dependencies
point exclusively inward.

```
cmd/dmarc-analyzer  →  internal/web  →  internal/app  →  internal/domain
                        internal/infra ─────────────────↗
```

## Layers

- **`internal/domain`** — pure business logic: aggregates
  (`report.AggregateReport`, `account.MailAccount`), value objects,
  validation, domain ports (interfaces like `report.Repository`,
  `sync.MessageSource`). No imports outside the standard library — no
  `encoding/xml`, no SQL, no `net/http` here.
- **`internal/app`** — use cases that orchestrate a business task
  (`syncreports.UseCase.SyncAccount`, `retention.UseCase.ApplyNow`,
  `manageaccount.UseCase.TestConnectionByID`). Depend only on domain
  ports, never on a concrete adapter.
- **`internal/infra`** — adapters that implement domain ports against
  concrete technology: `sqlite` (persistence), `imap` (IMAP access),
  `dmarcxml` (RFC 7489 parser), `mailmime` (MIME splitting), `sourceinfo`
  (PTR/service detection), `envconfig` (reads the entire runtime
  configuration from environment variables, see
  `docs/features/deployment.md`).
- **`internal/web`** — the delivery layer: `net/http` + `html/template`
  (no bundler, no Node tooling) for server-rendered pages, Chart.js for
  charts (`static/charts.js` receives already-prepared JSON data from Go
  and stays thin itself), plus the OIDC login flow (`auth.go`/`oidc.go`,
  see `docs/features/auth.md`). Calls exclusively into use cases from
  `internal/app`, never directly into an infra adapter.
- **`cmd/dmarc-analyzer`** — the composition root: the only place where
  concrete adapters are wired to use cases (`wire.go`); also reads the
  ENV configuration here (`envconfig.Load()`) and starts the appropriate
  path depending on the subcommand (`web`, `sync`, `import`, `stats`,
  `healthcheck`).

## Why this separation

- **Domain independently testable** — business rules (e.g. "when does a
  message count as DMARC-compliant", RFC 7489 field defaults) can be
  checked without a database, without network, without an HTTP server.
- **Swappable adapters** — `internal/app` knows only interfaces like
  `report.Repository`; tests plug in in-memory fakes there, production
  uses real SQLite. Switching databases would only affect
  `internal/infra/sqlite`, not a single line in
  `internal/domain`/`internal/app`.
- **A single place knows the concrete technology** — only
  `cmd/dmarc-analyzer` imports `internal/infra/sqlite`,
  `internal/infra/imap`, etc. *and* assembles the use cases from them.
  This keeps `internal/app`/`internal/web` free of technical details
  that don't belong there.

## Optional ports instead of large interfaces

Several ports are deliberately cut small and optional instead of being
bundled into one large interface — a caller checks via type assertion
whether an adapter has the extra capability:

- `domainsync.MailboxLister` — optional extension of
  `domainsync.MessageSource` that can list the mailboxes present on the
  server.
- `domainsync.MultiReportParser` — optional extension of
  `domainsync.ReportParser` for attachments with multiple reports (e.g.
  a `.zip` with several XML files).
- `report.Pruner` — its own narrow port just for the retention policy
  (`DeleteOlderThan`), separate from `report.Repository`, because
  deleting by age is a pure maintenance operation that only
  `internal/app/retention` needs.

## Sync pipeline (`internal/app/syncreports`)

Fetching and parsing run concurrently (a worker pool decodes/parses MIME
attachments in parallel), while writing to SQLite runs **serially** through
a single goroutine — SQLite doesn't tolerate concurrent writers well.
Progress (`sync.State`, last processed UID) is only persisted at a
gapless boundary, so a cancellation mid-run never skips a message, but at
most reprocesses an already-handled message on the next run (the `UNIQUE`
index on `reports` catches any resulting duplicates).

## Background jobs (`internal/app/syncjob`, `syncscheduler`, `retentionjob`)

Three independent, long-lived goroutines run alongside the HTTP server
(started in `cmd/dmarc-analyzer/cmd_web.go`, stopped via the same context
as the server):

- `syncjob.Runner` — at most one manually (`POST /abgleich`) or
  scheduler-triggered sync run at a time, progress via Server-Sent
  Events to any number of browser tabs.
- `syncscheduler.Scheduler` — triggers `syncjob.Runner.Start()` every
  `DMARC_SYNC_INTERVAL_MINUTES` (0 = disabled).
- `retentionjob.Runner` — applies the retention policy once at startup
  and then every 24h.
