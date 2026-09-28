# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Project scaffolding (WP 0): `go.mod`, directory structure following
  Clean Architecture layers, `Taskfile.yml`, `.golangci.yml`,
  GitHub Actions CI for macOS/Linux/Windows.
- Central logging (`internal/platform/logging`) and platform-appropriate
  path resolution (`internal/platform/paths`) for database, configuration,
  and logs.
- Composition root (`cmd/dmarc-analyzer`), starts and logs the version.
- Documentation of planned dependencies with pinned versions
  (`docs/DEPENDENCIES.md`).
- `README.md`, `LICENSE` (MIT), this changelog.
- Domain model for DMARC aggregate reports (WP 1): aggregate root
  `report.AggregateReport`, value objects (`SourceIP`, `DomainName`,
  `DateRange`, `Disposition`, `Policy`, `AlignmentMode`, `AuthResultValue`),
  `Record`, `PublishedPolicy` — all with enforced invariants and no
  dependencies outside the standard library. Unknown enum values seen in
  practice are mapped to `Unknown` instead of being discarded.
- `report.Repository` port (filtering/sorting/grouping, keyset pagination)
  and `sync.ReportParser` port, `sync.State` for the incremental sync.
- DMARC XML parser (`internal/infra/dmarcxml`): unpacks `.xml`, `.xml.gz`,
  and `.zip` (including multiple reports per archive), tolerates RFC
  deviations from individual providers (casing, missing
  `pct`/`adkim`/`aspf`), guards against zip/gzip bombs with a 100 MB limit.
- Golden-file tests against five synthetic provider fixtures, a fuzz test
  (`FuzzParse`), and a test proving external XML entities (XXE) are not
  resolved.
- SQLite persistence (WP 2): a custom migrator with embedded, numbered
  migrations; `internal/infra/sqlite.ReportRepository` fully implements
  `report.Repository` — `Save` (transaction, prepared batch statements),
  `Exists`/`ErrDuplicateReport` for deduplication by
  `(org_name, report_id, date_begin)`, `FindByID` (complete, including
  records), and `Query` (filtering, sorting, keyset pagination via SQLite
  row-value comparisons).
- Integration tests against a temporary file DB (WAL, real transactions)
  plus a performance test that imports 100,000 records and measures a
  typical report-table query against them (~7-8 ms).
- Mail & security (WP 3): `internal/domain/account` (aggregate
  `MailAccount`, type `Secret` with structurally enforced masking via
  `String()`/`GoString()`/`MarshalJSON()`, ports `Repository` and
  `CredentialStore`).
- IMAP adapter (`internal/infra/imap`) against
  `github.com/emersion/go-imap/v2`: context-aware TLS connection setup,
  `EXAMINE`+`BODY.PEEK[]` (mailbox stays untouched), UID-based streaming
  iterator, automatic rescan on `UIDVALIDITY` change, backoff with 3
  retries for connection setup (not for login). Tested against an
  in-process IMAP server (`imapmemserver`), 11 tests, 89% coverage.
- Keyring adapter (`internal/infra/keyring`): `OSStore` against the OS
  keychain (service `de.pmoscode.dmarc-analyzer`), `FileStore` as an
  AES-256-GCM fallback with scrypt key derivation for systems without a
  secret service, `IsAvailable()` for runtime detection.
- Application layer (WP 4): `syncreports.UseCase` orchestrates the
  complete incremental sync (connect, fetch, split MIME, parse,
  deduplicate, save, persist progress) as a concurrent pipeline
  (fetching/parsing in parallel, writing serial) with correct progress
  persistence even for out-of-order completed messages.
  `importfiles.UseCase` imports the same formats from local files (.eml,
  .xml, .xml.gz, .zip) and shares MIME splitting and deduplication with
  `syncreports`. `manageaccount.UseCase` fully manages accounts (create,
  connection test, delete including keychain entry). `statistics.UseCase`
  computes dashboard metrics including comparison to the previous period.
  `queryreports.UseCase` and `exportdata` (CSV export) round out the
  application layer.
- New SQLite adapters: `AccountRepository`, `SyncStateRepository`,
  `FailedImportRepository`, `StatisticsRepository` (SQL aggregation
  instead of loading individual records).
- MIME splitting (`internal/infra/mailmime`) of raw messages into
  attachments, against `github.com/emersion/go-message`.
- CLI (`cmd/dmarc-analyzer`): `sync`, `import <path>`, `stats`,
  `account add|list|test|delete`. The composition root wires adapters
  only for the subcommand actually invoked — `stats`/`import` never
  touch the OS keychain.
- `report.ErrDuplicate` and `report.SaveIfNew` as shared, domain-side
  deduplication logic for all import paths.
- UI scaffolding (WP 5, `internal/ui`): main window with a side
  navigation (reports/settings) and its own theme (accent color, light
  and dark via Fyne's `ThemeVariant` system). `internal/ui/i18n` bundles
  all visible text centrally. `settings.View` for creating, testing, and
  deleting accounts. Three-step first-run wizard (`onboarding.Wizard`:
  account → connection test → first sync). `reports.View` with a
  lazy-loading report table via `ReportQuery` (page size 50) and a
  detail view. Sync button with progress display and cancel, I/O
  exclusively via an injectable `runBackground` outside the UI thread,
  returning via `fyne.Do()`. Empty states and plain-text error messages
  for every view. Fully tested with `fyne.io/fyne/v2/test` (layout,
  navigation, form validation), `-race`-clean thanks to injectable
  `runBackground`.
- Fixed a real bug: `settings.View.refreshContent()` overwrote the header
  (title + add button) instead of the list on every `Reload()`, because
  in `container.NewBorder` the center slot sits at index 0, not at the
  end of `.Objects` — uncovered by the navigation test
  `TestShell_SelectNav_SwitchesToSettings`.
- Analysis and visualization (WP 6): overview (`internal/ui/dashboard`)
  with metric tiles including a trend arrow vs. the previous period, plus
  four charts (time series stacked by pass/fail, top 10 sending sources
  colored by volume and pass rate, disposition donut, source-×-day
  heatmap). `analysis.ChartRenderer` port implemented against
  `github.com/wcharczuk/go-chart/v2` (`internal/infra/charts`); the
  heatmap is drawn directly with `image/draw` for lack of go-chart
  support. `analysis.Repository` extended with
  `DailyVolumes`/`TopSources`/`Heatmap` (SQL aggregation like
  `Compute`), `statistics.UseCase.Dashboard` bundles all overview data
  into a single load.
- Sending-sources view (`internal/ui/sources`): a lazy-loading table
  aggregated by source IP (volume, pass rate, PTR hostname, detected
  service) via the new domain port `domain/sources`
  (`SourceStatsRepository` in `internal/infra/sqlite`, keyset pagination
  like reports). rDNS/PTR resolution with a process cache, plus
  hostname-based detection of known service providers (Google Workspace,
  Microsoft 365, Mailchimp, SendGrid, Brevo, Postmark) in
  `internal/infra/sourceinfo` — deliberately without additional IP-range
  lists, see UMSETZUNGSPLAN.md for the rationale.
- A shared filter bar (`components.FilterBar`: time-range preset +
  domain) now acts on the overview, reports, and sources views at once.
  `reports.View` also got its own grouping `Select` (none/domain/
  organization), connecting `report.Query.GroupBy` — present since WP 2 —
  to the UI for the first time.
- Glossary (`internal/ui/glossary`) with the most important DMARC terms,
  reachable via a header button plus small "?" buttons on individual
  metric tiles — a substitute for hover tooltips, which Fyne v2.8 doesn't
  support.
- Export: `exportdata.WriteChartPNG` (charts as PNG) and
  `exportdata.WriteSourceStatsCSV` (sending sources as CSV) round out the
  existing report/record CSV export; `reports.View`/`sources.View`/
  every chart panel each have their own export button.
- The GUI program is now actually launchable: `dmarc-analyzer` with no
  arguments opens the main window (`cmd_gui.go`), `--help`/`-h` shows the
  command-line help. Previously, `ui.BuildMainWindow` existed fully
  tested but was wired up nowhere — a gap open since WP 5.
- Fixed a real bug (construction order): `widget.Select.SetSelected()`
  fires `OnChanged` synchronously, even from the constructor. The new
  grouping `Select` in `reports.View` was thus accessing the not-yet-
  assigned `v.container` via the prematurely fired handler during the
  `NewView()` call (nil-pointer panic) — uncovered by a simple
  construction test.
- Folder picker for the account form: DMARC reports don't necessarily
  land in the root mailbox — a new, optional port `sync.MailboxLister`
  (implemented by `imap.Adapter` via IMAP LIST, excluding `\Noselect`
  mailboxes) plus `manageaccount.UseCase.ListMailboxes` let the account
  form (settings and first-run wizard) list the mailboxes/subfolders
  that actually exist on the server. The mailbox field is now a
  `widget.SelectEntry` (still freely typable, plus a dropdown of the
  discovered folders).
- A more modern, fully custom color scheme (`internal/ui.appTheme`) for
  light and dark mode, following the validated reference palette of the
  `dataviz` skill (primary blue, fixed status colors green/yellow/red) —
  fixes the reported "dated, all black" look in dark system mode, which
  previously delegated almost entirely to Fyne's generic default theme.
  The same status colors are now also used in `internal/infra/charts`
  (previously ad-hoc chosen green/red tones) — the UI and charts now
  speak the same color language. Larger corner radii/spacing (cards,
  buttons, input fields) for a more airy, less blocky feel.
- Dashboard tiles and charts now live inside `widget.Card` instead of
  standing freely on the window background, with an icon per metric.
  The four charts are now regrouped instead of one long column: time
  series and disposition donut side by side, top sending sources and
  heatmap each getting their own full row with a horizontal scroll area
  (their width grows with the number of sending sources/days). Side
  navigation now has an icon per entry, dividers between header/
  navigation and the content.
- Fixed a real bug: the report detail dialog (report table, tap a row)
  could no longer be closed. It ran on `dialog.NewCustomWithoutButtons` —
  with no button at all, and no code path called `Hide()`. Unlike a
  regular popup, the underlying `widget.ModalPopUp` doesn't close on a
  tap outside it, so the dialog stayed open permanently. Now uses
  `dialog.NewCustom(...)` with a "Close" button.
- `DMARC_OIDC_INSECURE_SKIP_VERIFY` (optional, default `false`): disables
  TLS certificate verification for all calls to the OIDC issuer
  (discovery, JWKS, token exchange) — intended exclusively for
  development environments with a self-signed certificate (e.g. Caddy's
  `tls internal`), never for production. See
  `docs/features/deployment.md`.

### Changed

- Migrated the entire presentation layer from the Fyne desktop UI to a
  web UI embedded in the program (`internal/web`, `MIGRATIONSPLAN.md`
  milestones M0–M5): on startup, a local HTTP server runs on
  `127.0.0.1`, and the default browser automatically opens a page with a
  one-time login link (valid for 60 s, exchanged for an
  `HttpOnly`/`SameSite=Strict` session cookie). Single-instance detection
  via `instance.json`: a second launch just fetches a fresh login link.
  Security model: `Host` check against DNS rebinding, CSRF token and
  `Origin`/`Sec-Fetch-Site` checks on every state-changing request,
  Content Security Policy without `unsafe-inline`. Lifecycle exclusively
  via Ctrl+C/SIGTERM (no auto-exit, no "quit" button) — see README
  "Start and stop".
- All four dashboard charts (time series, top sending sources,
  disposition, heatmap) are now drawn client-side with Chart.js
  (`internal/web/static/charts.js`, plugins `chartjs-chart-matrix` for
  the heatmap and `chartjs-plugin-zoom` for the time series) instead of
  server-side as PNG (`go-chart`) — this brings tooltips, a toggleable
  legend, zoom in the time series, and click drill-down (clicking a
  day/source/cell/segment opens the correspondingly filtered reports).
  Each chart also has a toggleable table view (accessibility) plus
  PNG/CSV export buttons.
- Report and sending-source tables now use paged lazy-loading via htmx
  instead of a virtualized Fyne table; CSV export streams the entire
  filtered dataset instead of holding it fully in memory.
- Import (`importfiles.UseCase.ImportData`) now also works via web
  upload (`POST /import`, drag & drop, multiple files at once, 50 MB per
  file) instead of only via a CLI file path.
- Sync now runs as a server-side background job
  (`internal/app/syncjob`, at most one run at a time, cancellable via
  context) with live progress over Server-Sent Events
  (`GET /ereignisse`) instead of a synchronous Fyne progress dialog.
- Credential-store lock: `account.CredentialStore` can now be locked
  (`account.ErrCredentialStoreLocked`) — the UI then redirects to an
  unlock page (`/entsperren`) for the master passphrase of the file
  fallback key store (Linux without a secret service), instead of
  prompting on the console as before (which doesn't exist when started
  without a terminal, e.g. via double-click).

### Removed

- `internal/ui` (the entire Fyne UI, ~4,900 lines including tests),
  `cmd/dmarc-analyzer/cmd_gui.go`, and `fyne.io/fyne/v2` plus all
  transitive Fyne dependencies from `go.mod` (milestone M5). The hidden
  transitional subcommand `gui` (comparison/fallback path during the
  migration) goes away with it as well.
- `internal/infra/charts` (the `go-chart` implementation of the
  `ChartRenderer` port), `analysis.ChartRenderer` itself, and
  `exportdata.WriteChartPNG` — charts are now produced entirely in the
  browser (see "Changed" above). `github.com/wcharczuk/go-chart/v2`
  removed from `go.mod` as well. Rationale in
  `docs/adr/0001-web-oberflaeche-statt-fyne.md` and
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.
- The whole module is now free of CGO dependencies
  (`go list -deps ./... | grep fyne` returns nothing anymore); `task
  release` builds all target platforms (macOS arm64/amd64, Windows
  amd64, Linux amd64/arm64) as pure `CGO_ENABLED=0` cross-compiles on a
  single machine, without `fyne package` or platform-specific
  toolchains — see `Taskfile.yml` (`release:darwin`/`release:windows`/
  `release:linux`/`release`) and the new GitHub Actions pipeline
  (`.github/workflows/ci.yml`: `task check` on every push/PR, `task
  release` including a GitHub release on version tags).

### Fixed

- Access denial for missing admin group membership showed only a bare
  plain-text sentence (`http.Error`, no HTML, no styling) instead of a
  comprehensible error page. New standalone page `access_denied.html`
  (without the app layout, since no session exists at this point) names
  the affected account, explains the cause, and offers a link to try
  logging in again.
- `/abmelden` (logout) had no effect: the redirect to Authentik's
  `end_session_endpoint` (RP-initiated logout) is necessarily a
  different origin, but was blocked by the browser due to our own CSP
  header (`form-action 'self'`). The local session was ended, but
  Authentik was never told about the logout — the next request
  (`GET /anmelden` redirects straight to Authentik with no intermediate
  step) immediately logged back in via the still-active Authentik
  session, with nothing visibly happening. `form-action` now also
  allows the origin of the configured OIDC issuer
  (`internal/web/middleware.go:buildContentSecurityPolicy`).
- `.zip` attachments containing multiple DMARC reports were only
  imported as a single report, both during file import and IMAP sync
  (`ReportParser.Parse()` instead of `ParseAll()`) — fixed via a new,
  optional interface `domainsync.MultiReportParser` and a shared helper
  function `domainsync.ParseAttachment`.
- `syncjob.Runner` incorrectly reported a cancellation during the last
  account as `done`.
- `/entsperren` accepted any passphrase when no account existed yet.
- A Chart.js chart (message volume per day) grew unbounded downward
  while the dashboard was loading — a classic Chart.js feedback-loop bug
  with `responsive: true` + `maintainAspectRatio: false` without a
  wrapper element that has a fixed height independent of the canvas
  (see `AGENTS.md`).
