# Implementation plan — DMARC Analyzer

> Basis: `FEATURES.md`. This plan turns the requirements into a concrete
> architecture and phase plan.

---

## 1. Starting point and interpretation of the requirements

### 1.1 Terminology clarification

`FEATURES.md` consistently says "DMerc". The repository is called
`dmarc-analyzer`, and the described flow (fetch data from a mail
account, evaluate it, visualize it) matches the DMARC reporting workflow
exactly. **Assumption: DMARC is meant** (Domain-based Message
Authentication, Reporting and Conformance, RFC 7489).

### 1.2 On the "Non features" section

The "## Non features" section in `FEATURES.md` in substance lists
**technical requirements** (Go, Fyne, secure credentials, Taskfile,
tests, README, CHANGELOG, SOLID/DDD/Clean Architecture/Clean Code).
These are treated here as **binding requirements**, not as exclusions.
If exclusions were actually meant: please say so, the plan would then
change fundamentally.

### 1.3 Decisions clarified

| Topic | Decision |
| --- | --- |
| Report types v1 | Only **DMARC Aggregate Reports (RUA)**. Interfaces deliberately generic, so RUF and TLS-RPT can plug in later without a rewrite. |
| Storage | **SQLite via `modernc.org/sqlite`** (pure Go, no CGO → cross-compiling stays simple). |
| Mail access v1 | **IMAP with username/password or app password.** |
| Language | **Consistently German**: UI, README, CHANGELOG, code comments, commit messages. |

**Exception to the language rule:** Go identifiers (package, type,
function, field names) stay **English**. German identifiers with
umlauts break Go conventions, hurt readability alongside the standard
library and Fyne, and make later contribution harder. Everything humans
read — comments, user-facing error text, log messages, UI strings, docs
— is German. This can be changed if desired; please just say so.

> **Historical note:** this decision was later reversed — the whole
> codebase, including comments/errors/logs/UI strings/docs, now uses
> English throughout (see `AGENTS.md`'s language rule). This document is
> kept unchanged as historical context and does not reflect that later
> change.

---

## 2. Target picture

A standalone desktop program (macOS, Windows, Linux) as a single binary:

1. Connects to a configured IMAP mailbox that receives DMARC aggregate
   reports.
2. Fetches **incrementally**, only new messages, unpacks the attachments
   (`.gz`, `.zip`, plain `.xml`), parses the feedback XML.
3. Stores the normalized data locally in SQLite — idempotently,
   duplicates are detected and discarded.
4. Presents the data in a Fyne UI: dashboard with metrics and charts,
   filterable/sortable/groupable tables, detail views.
5. Stores credentials in the OS keychain, never in plaintext.

---

## 3. Technology stack

> **M5 addendum (see `MIGRATIONSPLAN.md`,
> `docs/adr/0001-web-oberflaeche-statt-fyne.md`,
> `docs/adr/0002-chartjs-statt-chartrenderer-port.md`):** the originally
> planned rows "UI" (Fyne), "Charts" (go-chart), and "UI tests" (Fyne
> test driver) below have been replaced by an embedded web UI —
> rationale and consequences in the two ADRs.

| Purpose | Library | Rationale |
| --- | --- | --- |
| UI | `internal/web`: `net/http` + `html/template` (standard library) + `htmx` | Embedded web UI in the default browser instead of Fyne — no bundler, no Node tooling, `go build` stays the only build step (ADR 0001). |
| IMAP | `github.com/emersion/go-imap/v2` | Current, IMAP4rev1+rev2, clean API. |
| MIME/attachments | `github.com/emersion/go-message` | Robust parsing of multipart mail and encodings. |
| Database | `modernc.org/sqlite` (via `database/sql`) | CGO-free, cross-compiles without a C toolchain. |
| Credentials | `github.com/zalando/go-keyring` | macOS Keychain, Windows Credential Manager, Linux Secret Service. |
| Charts | Chart.js + `chartjs-chart-matrix` + `chartjs-plugin-zoom` (`internal/web/static/vendor/`, minified UMD files checked into the repository instead of a CDN) | Rendered in the browser, interactive (tooltip, toggleable legend, zoom, click drill-down); the server delivers only JSON via `/api/diagramme/*`. Replaces the originally planned `ChartRenderer` port (see 8.2, ADR 0002). |
| Logging | `log/slog` (standard library) | No extra dependency, structured logs. |
| XML/archives | `encoding/xml`, `compress/gzip`, `archive/zip` | The standard library is entirely sufficient. |
| Tests | `testing` + `github.com/stretchr/testify/require` | Table-driven tests, terse assertions. |
| Web tests | `net/http/httptest` (standard library) | Handler and template tests without a real server/browser — every page is rendered and parsed as HTML (see 12.2). |
| Task runner | `Taskfile.yml` (go-task) | Requirement; `task` is already installed locally. |
| Linting | `golangci-lint` | Bundles vet, staticcheck, errcheck, revive, etc. |

**Go version:** 1.27 (1.27.1 available locally).
**Module path:** `github.com/pmoscode/dmarc-analyzer`.

Dependencies are kept deliberately lean: anything the standard library
handles cleanly isn't replaced by a dependency. Concrete pinned versions
(including the frontend files under `internal/web/static/vendor/`) live
in `docs/DEPENDENCIES.md`.

Deferred: a `chromedp` browser smoke test for the four Chart.js charts
(MIGRATIONSPLAN.md decision E-8) — unwritten so far, because no
Chrome/Chromium was available in the development environments used so
far; see `MIGRATIONSPLAN.md` section 11.

---

## 4. Architecture

### 4.1 Layer model (Clean Architecture)

Dependencies point exclusively **inward**. The domain layer knows
neither SQL, nor IMAP, nor the web UI.

```
┌───────────────────────────────────────────────────────────┐
│  cmd/dmarc-analyzer  — composition root, wiring, startup   │
├───────────────────────────────────────────────────────────┤
│  internal/web        — html/template + Chart.js (browser) │
│  internal/infra      — IMAP, SQLite, keyring, XML          │
├───────────────────────────────────────────────────────────┤
│  internal/app        — use cases                           │
├───────────────────────────────────────────────────────────┤
│  internal/domain     — entities, value objects, ports       │
└───────────────────────────────────────────────────────────┘
```

* **domain** — pure business logic. No imports outside the standard
  library. Defines the *ports* (interfaces) implemented on the outside.
* **app** — orchestrates use cases, knows only domain ports.
* **infra** — *adapters*: implement the ports against concrete
  technology.
* **web** — web UI (`html/template` + Chart.js), calls exclusively into
  use cases (originally planned as a Fyne desktop UI, see
  `MIGRATIONSPLAN.md`/ADR 0001 on the migration).
* **cmd** — the only place where concrete implementations are wired up.

### 4.2 Relation to the required principles

**SOLID**
* *SRP* — one responsibility per package; the parser parses, the
  repository persists, the fetcher fetches.
* *OCP* — new report types (RUF, TLS-RPT) are added as an additional
  `ReportParser` implementation, without changing existing code.
* *LSP* — ports are behaviorally defined; fakes in tests behave like the
  real adapters.
* *ISP* — narrow interfaces (`ReportReader`, `ReportWriter` instead of
  one fat `ReportStore`).
* *DIP* — `app` depends on interfaces from `domain`, never on `infra`.

**DDD**
* *Aggregate root*: `AggregateReport` — records exist only within a
  report and are loaded and saved exclusively through it.
* *Value objects* (immutable, no identity): `SourceIP`, `DomainName`,
  `DateRange`, `Disposition`, `AlignmentMode`, `PolicyEvaluation`,
  `AuthResults`.
* *Domain services*: `AlignmentEvaluator` (evaluates alignment),
  `ReportDeduplicator`.
* *Repositories*: one per aggregate, with business language
  (`FindByDomainAndPeriod`, not `SelectWhere`).
* *Ubiquitous language*: terms from RFC 7489 are adopted 1:1
  (disposition, alignment, policy published, header from, …) — no
  invented terminology.

**Clean Code**
* Functions short and at one level of abstraction, meaningful names, no
  flag parameters.
* Errors are contextualized with `fmt.Errorf("...: %w", err)` and never
  swallowed.
* Comments explain the *why*, not the *what*.
* No global state outside the composition root.

---

## 5. Project structure

> **M5 addendum:** `internal/ui` (Fyne) has been fully removed,
> `internal/infra/charts` (go-chart) as well — replaced by `internal/web`
> (see `MIGRATIONSPLAN.md`, ADR 0001/0002). The tree below reflects the
> current state.

```
dmarc-analyzer/
├── cmd/
│   └── dmarc-analyzer/
│       ├── main.go                  # entry point
│       └── wire.go                  # composition root: wires up adapters
├── internal/
│   ├── domain/
│   │   ├── report/                  # aggregate: AggregateReport
│   │   │   ├── report.go
│   │   │   ├── record.go
│   │   │   ├── policy.go
│   │   │   ├── valueobjects.go
│   │   │   └── repository.go        # port: ReportRepository
│   │   ├── account/                 # aggregate: MailAccount
│   │   │   ├── account.go
│   │   │   ├── credentials.go
│   │   │   └── repository.go        # ports: AccountRepository, CredentialStore
│   │   ├── sync/
│   │   │   ├── state.go             # SyncState per account/mailbox
│   │   │   ├── failedimport.go      # failed imports (log, retry)
│   │   │   └── ports.go             # ports: MessageSource, ReportParser, MultiReportParser
│   │   ├── sources/                 # sending-source enrichment (service detection, PTR)
│   │   └── analysis/
│   │       ├── statistics.go        # metrics, aggregations for the dashboard
│   │       └── charts.go            # chart data types (DailyVolume, Heatmap, …) for /api/diagramme/*
│   ├── app/
│   │   ├── syncreports/             # use case: fetch + import reports
│   │   ├── syncjob/                 # use case: sync as a server-side job (at most one run at a time)
│   │   ├── queryreports/            # use case: filter, sort, group
│   │   ├── statistics/              # use case: dashboard metrics
│   │   ├── sourcestats/             # use case: sending-source statistics
│   │   ├── manageaccount/           # use case: create/test/delete account
│   │   ├── importfiles/             # use case: import from file/bytes (.eml/.xml/.zip)
│   │   └── exportdata/              # use case: CSV export (streamed)
│   ├── infra/
│   │   ├── imap/                    # MessageSource adapter
│   │   ├── dmarcxml/                # ReportParser adapter (XML + gz/zip)
│   │   ├── mailmime/                # MIME splitting of mailbox messages
│   │   ├── sourceinfo/              # PTR/service detection for sending sources
│   │   ├── sqlite/
│   │   │   ├── migrations/          # *.sql, embedded via go:embed
│   │   │   ├── db.go
│   │   │   ├── migrate.go
│   │   │   ├── reportrepo.go
│   │   │   ├── accountrepo.go
│   │   │   └── syncstaterepo.go
│   │   ├── keyring/                 # CredentialStore adapter (OS keychain + file fallback)
│   │   └── config/                  # settings (non-secret)
│   ├── web/
│   │   ├── server.go, routes.go     # http.Server, handler tree, middleware wiring
│   │   ├── middleware.go            # CSP/security headers, host check, CSRF, session
│   │   ├── auth.go, instance.go     # one-time login link, session cookie, instance.json
│   │   ├── handlers_*.go            # one handler per page/action (dashboard, reports, sources, import, export, accounts, sync, …)
│   │   ├── api_charts.go            # JSON endpoints for the four Chart.js charts
│   │   ├── templates/               # html/template (layout.html + pages/*.html)
│   │   ├── static/                  # app.css, app.js, charts.js, vendor/ (htmx, Chart.js, plugins)
│   │   └── glossary/                # central term explanations (German)
│   └── platform/
│       ├── logging/
│       └── paths/                   # paths for DB, logs, config
├── testdata/
│   └── reports/                     # real example reports (anonymized)
├── docs/
│   ├── adr/                         # Architecture Decision Records
│   └── DEPENDENCIES.md
├── packaging/
│   └── darwin/Info.plist.tmpl       # minimal .app bundle manifest for "task release:darwin"
├── .github/workflows/                # CI: fmt+lint+test, cross-compiled release on tags
├── Taskfile.yml
├── README.md
├── CHANGELOG.md
├── FEATURES.md
├── IMPLEMENTIERUNG.md
├── go.mod
└── .golangci.yml
```

`internal/` prevents internals from accidentally becoming a public API.

---

## 6. Domain model

### 6.1 Aggregate `AggregateReport`

Derived from the schema in RFC 7489, Appendix C.

```go
// AggregateReport is the aggregate root of a DMARC report.
// Records exist only within the context of their report.
type AggregateReport struct {
    ID          ReportID
    Metadata    Metadata        // sending org, report ID, period
    Policy      PublishedPolicy // published DMARC policy
    Records     []Record        // evaluated sending sources
    ImportedAt  time.Time
    SourceRef   SourceReference // origin: account, mailbox, UID, filename
}

type Metadata struct {
    OrgName          string
    Email            string
    ExtraContactInfo string
    ReportID         string
    Range            DateRange
    Errors           []string
}

type PublishedPolicy struct {
    Domain          DomainName
    SubdomainPolicy Policy         // none | quarantine | reject
    Policy          Policy
    DKIMAlignment   AlignmentMode  // r (relaxed) | s (strict)
    SPFAlignment    AlignmentMode
    Percentage      int
    FailureOptions  string
}

type Record struct {
    SourceIP    SourceIP
    Count       int
    Evaluated   PolicyEvaluation // disposition + dkim/spf result + reasons
    Identifiers Identifiers      // header_from, envelope_from, envelope_to
    Auth        AuthResults      // individual DKIM and SPF results
}
```

### 6.2 Business invariants (enforced in constructors)

* A report without a `ReportID` or without a valid period is invalid.
* `DateRange.Begin` is before `DateRange.End`.
* `Count` per record is `> 0`.
* `Percentage` is in `[0, 100]`.
* Unknown enum values from the XML are mapped to `Unknown`, not
  discarded — providers don't always follow the RFC.

### 6.3 Business identity and deduplication

The business identity of a report is the triple
`(OrgName, ReportID, DateRange.Begin)`. A UNIQUE index sits on it. This
makes the import idempotent even if the same mail is fetched twice —
e.g. after a `UIDVALIDITY` change on the server or a mailbox restore.

### 6.4 Ports (defined in `domain`, implemented in `infra`)

```go
// MessageSource delivers raw messages from a source (v1: IMAP).
// Deliberately technology-neutral, so file import or other protocols
// can plug in later without changing the use cases.
type MessageSource interface {
    Connect(ctx context.Context, acc account.MailAccount) error
    FetchNew(ctx context.Context, state SyncState) (iter.Seq2[RawMessage, error], SyncState, error)
    Close() error
}

// ReportParser turns a raw attachment into domain objects.
// Supports() selects the matching implementation — this is how RUF and
// TLS-RPT get added additively later (Open/Closed).
type ReportParser interface {
    Supports(attachment RawAttachment) bool
    Parse(ctx context.Context, attachment RawAttachment) (*AggregateReport, error)
}

type ReportRepository interface {
    Save(ctx context.Context, r *AggregateReport) error
    Exists(ctx context.Context, key ReportKey) (bool, error)
    FindByID(ctx context.Context, id ReportID) (*AggregateReport, error)
    Query(ctx context.Context, q ReportQuery) (ReportPage, error)
}

type CredentialStore interface {
    Store(accountID string, secret []byte) error
    Retrieve(accountID string) ([]byte, error)
    Delete(accountID string) error
}
```

`FetchNew` returns an iterator instead of a slice: large mailboxes are
thus processed in a streaming fashion, without holding all messages in
memory.

---

## 7. Core flow: incremental sync

> Requirement: "It is smart to discover new data only in the mail account."

### 7.1 Flow

```
Use case SyncReports
  │
  ├─ 1. Load account, fetch password from the keychain
  ├─ 2. Establish the IMAP connection (TLS enforced)
  ├─ 3. Select mailbox, load SyncState
  │      ├─ UIDVALIDITY matches?  → continue UID-based from LastUID+1
  │      └─ UIDVALIDITY changed?  → full rescan,
  │                                  the UNIQUE index catches duplicates
  ├─ 4. UID FETCH (BODY.PEEK[]) the new messages, streamed
  │      └─ PEEK: the \Seen flag stays untouched, the mailbox
  │               isn't "consumed" for other clients
  ├─ 5. Per message:
  │      ├─ Split MIME, extract attachments
  │      ├─ Decompress (.gz / .zip / plain .xml)
  │      ├─ Pick the matching ReportParser and parse
  │      ├─ Check the deduplication key → skip if needed
  │      └─ Save in one transaction (report + all records)
  ├─ 6. Persist SyncState after every successful message
  │      └─ A cancellation/crash costs at most one message reprocessed
  └─ 7. Report the result: new / skipped / failed
```

### 7.2 Robustness

* **Error quarantine** — unparseable attachments land with an error
  message in `failed_imports`, instead of aborting the whole run.
  Visible in the UI and retryable individually.
* **Raw-data archive** — the original XML is optionally stored
  gzip-compressed in `raw_reports`. This lets it be re-read after a
  parser fix, without querying the mailbox again. Can be disabled
  (storage).
* **Context cancellation** — every step respects `context.Context`; the
  sync button in the UI can be cancelled at any time.
* **Backoff** — exponentially staggered retry (3 attempts) on transient
  IMAP errors, then a clean abort with a comprehensible message.
* **Transactions** — a report is saved fully or not at all.

### 7.3 Concurrency

Fetching (I/O-bound) and parsing (CPU-bound) run as a pipeline over
channels with a bounded number of workers (`GOMAXPROCS`, capped). The
write operations run **serially** in a single goroutine — SQLite doesn't
like concurrent writers. In addition: `journal_mode=WAL`,
`busy_timeout=5000`.

---

## 8. Persistence

### 8.1 Schema (excerpt)

```sql
CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL,
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL,
    username      TEXT NOT NULL,
    mailbox       TEXT NOT NULL DEFAULT 'INBOX',
    use_tls       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL
    -- password deliberately NOT here, but in the keychain
);

CREATE TABLE sync_state (
    account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    mailbox       TEXT NOT NULL,
    uid_validity  INTEGER NOT NULL,
    last_uid      INTEGER NOT NULL,
    last_sync_at  TEXT,
    PRIMARY KEY (account_id, mailbox)
);

CREATE TABLE reports (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_name      TEXT NOT NULL,
    org_email     TEXT,
    report_id     TEXT NOT NULL,
    date_begin    INTEGER NOT NULL,   -- Unix time, UTC
    date_end      INTEGER NOT NULL,
    policy_domain TEXT NOT NULL,
    policy_p      TEXT, policy_sp TEXT,
    policy_adkim  TEXT, policy_aspf TEXT,
    policy_pct    INTEGER, policy_fo TEXT,
    account_id    TEXT REFERENCES accounts(id) ON DELETE SET NULL,
    message_uid   INTEGER,
    imported_at   TEXT NOT NULL,
    UNIQUE (org_name, report_id, date_begin)
);

CREATE TABLE records (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id     INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    source_ip     TEXT NOT NULL,
    message_count INTEGER NOT NULL,
    disposition   TEXT NOT NULL,
    dkim_result   TEXT NOT NULL,      -- evaluated (aligned)
    spf_result    TEXT NOT NULL,
    header_from   TEXT NOT NULL,
    envelope_from TEXT,
    envelope_to   TEXT
);

CREATE TABLE record_reasons     (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 type TEXT, comment TEXT);
CREATE TABLE auth_results_dkim  (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 domain TEXT, selector TEXT, result TEXT, human_result TEXT);
CREATE TABLE auth_results_spf   (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 domain TEXT, scope TEXT, result TEXT);
CREATE TABLE raw_reports        (report_id INTEGER PRIMARY KEY REFERENCES reports(id) ON DELETE CASCADE,
                                 filename TEXT, content BLOB);
CREATE TABLE failed_imports     (id INTEGER PRIMARY KEY AUTOINCREMENT, account_id TEXT, message_uid INTEGER,
                                 filename TEXT, error TEXT, raw BLOB, occurred_at TEXT NOT NULL);

CREATE INDEX idx_reports_period   ON reports(date_begin, date_end);
CREATE INDEX idx_reports_domain   ON reports(policy_domain);
CREATE INDEX idx_records_report   ON records(report_id);
CREATE INDEX idx_records_ip       ON records(source_ip);
CREATE INDEX idx_records_from     ON records(header_from);
```

Timestamps for report periods are stored as **Unix seconds in UTC** —
timezone logic belongs in the presentation layer, not in the data.

### 8.2 Further persistence decisions

* **Migrations** — numbered `.sql` files in
  `internal/infra/sqlite/migrations/`, embedded into the binary via
  `go:embed`, applied by a small custom migrator (~60 lines) with a
  `schema_migrations` table. No extra dependency.
* **PRAGMAs on open** — `journal_mode=WAL`, `foreign_keys=ON`,
  `busy_timeout=5000`, `synchronous=NORMAL`.
* **Performance** — batch inserts of records via prepared statements
  inside one transaction. `modernc.org/sqlite` is slower than the CGO
  variant; at typical report sizes (a few hundred records) that's
  irrelevant, but batching makes a difference on the initial import of
  large archives.
* **Storage location** — `os.UserConfigDir()/dmarc-analyzer/dmarc.db`,
  platform-appropriate.
* **~~ChartRenderer as a port~~ (removed in M5)** — originally,
  `analysis.ChartRenderer` was meant to return `image.Image` (v1
  implemented with `go-chart`), intended as a swappable adapter for
  later native, interactive Fyne widgets. With the move to the web UI,
  the motivation for that disappears entirely: charts are client-side in
  the browser anyway (Chart.js), a server-side image-renderer port is no
  longer needed. The plain data types
  (`analysis.DailyVolume`/`SourceVolume`/`Heatmap`/`HeatmapCell`) remain
  and are now delivered directly as JSON to `/api/diagramme/*`. Rationale
  and alternatives weighed: `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.

---

## 9. Secure handling of credentials

> Requirement: "The credentials are securely stored."

| Rule | Implementation |
| --- | --- |
| Password never in the DB | Only `accounts` metadata in SQLite; the secret lives in the OS keychain under service `de.freie-schule.dmarc-analyzer`, key = account ID. |
| Password never in the log | A dedicated `Secret` type with `String()`/`MarshalJSON()` that return `"***"` — leaks via `%v` or JSON dumps are thus structurally excluded. |
| Transport encrypted | IMAPS (port 993) as the default; STARTTLS as an alternative. Plaintext IMAP is only possible with explicit confirmation and is warned about in the UI. |
| Certificates | Regular verification against the system trust store. `InsecureSkipVerify` doesn't exist as an option. |
| Short lifetime in memory | The secret is only read right before login and overwritten with `Zero()` afterward. |
| Linux fallback | Without a secret service (D-Bus), an encrypted file store kicks in: AES-256-GCM, key derived from a master passphrase via `scrypt`. Prompted for at program startup. |
| Deleting means deleting | Deleting an account removes the metadata **and** the keychain entry. |

---

## 10. UI concept (web UI)

> **M5 addendum:** originally planned as a Fyne desktop UI (main window
> with side navigation, `widget.Table`, etc.) — replaced by the migration
> to a web UI running in the browser (`internal/web`, see
> `MIGRATIONSPLAN.md`, ADR 0001). The business-level views and metrics
> (10.1–10.3) stayed the same in substance, only the implementation
> (10.4) is completely different.

### 10.1 Navigation

Header navigation, marked active server-side based on the requested path
(`internal/web/handlers_nav.go`, no JavaScript needed):

| View | Route | Content |
| --- | --- | --- |
| **Overview** | `/` | Metric tiles, time series, top sending sources, distribution by disposition, source × day heatmap. |
| **Reports** | `/berichte`, `/berichte/{id}` | Paginated table of all reports; filter by time range, domain, sending org, source IP; CSV export of the entire filtered dataset. Clicking opens the detail view (metadata, published policy, record table with auth results). |
| **Sending sources** | `/quellen` | Aggregated by source IP: volume, pass rate, PTR/rDNS, detected service; CSV export. |
| **Import** | `/import` | Drag-and-drop/file upload for `.eml`/`.xml`/`.xml.gz`/`.zip`, result display (new/skipped/failed). |
| **Glossary** | `/glossar` | Central term explanations (German), linked from "?" links on other pages. |
| **Settings** | `/einstellungen` | Manage accounts, test connection, trigger/cancel mailbox sync. |
| **First-run setup** | `/einrichtung` | Create the first account — automatic redirect as long as no account exists. |
| **Unlock** | `/entsperren` | Master passphrase for the file key store (Linux without a secret service). |

### 10.2 Metrics on the overview

* Total number of evaluated messages in the selected time range
* DMARC pass rate (share of messages with `dkim=pass` **or** `spf=pass`
  after alignment)
* SPF alignment rate and DKIM alignment rate, reported separately
* Number of distinct sending sources
* Volume by disposition: `none` / `quarantine` / `reject`
* Change vs. the previous period (trend arrow)

### 10.3 Visualizations

* **Time series** — message volume per day, stacked by pass/fail.
* **Bars** — top sending sources by volume, colored by pass rate.
* **Donut** — distribution of dispositions.
* **Heatmap** — source × day, color = pass rate. Makes failing services
  immediately visible.
* **Table with grouping** — collapsible by domain, org, or source IP.

All four charts are interactive (Chart.js in the browser, see ADR 0002):
tooltip on hover, toggleable legend, zoom/pan in the time series, click
opens the correspondingly filtered reports (drill-down). Each chart also
has a toggleable table view (for screen readers, since `<canvas>` itself
isn't accessible) plus PNG/CSV export buttons.

### 10.4 Web practice

* **No inline JavaScript** — the Content Security Policy forbids
  `unsafe-inline`; every behavior lives in `internal/web/static/*.js`
  (see `AGENTS.md`).
* **Chart logic belongs in Go** — aggregation, filtering, and drill-down
  targets are produced server-side; `charts.js` receives already-prepared
  JSON data and stays deliberately thin (pure presentation).
* **Every state change via POST with a CSRF token** — never a GET with a
  side effect (see `internal/web/middleware.go: requireCSRF`).
* **Paged lazy-loading instead of virtualization** — unlike a Fyne
  `widget.Table`, the web table explicitly loads pages (keyset
  pagination from SQLite, `LIMIT`-like but without an expensive `OFFSET`
  count on large tables), also for CSV exports over the entire filtered
  dataset (streamed, see `internal/app/exportdata`).
* **Color palette for light and dark** via CSS variables
  (`internal/web/static/app.css`), the same values as in the Chart.js
  charts; pass/fail colors additionally distinguishable by icon/text
  (accessibility) — the palette follows the `dataviz` skill's reference
  palette, see `AGENTS.md`.
* **Progress display** during sync with the ability to cancel, via
  Server-Sent Events (`GET /ereignisse`) instead of polling.
* A desktop notification after a background sync finishes (originally
  planned with `fyne.App.SendNotification`) is still open with the move
  to the web UI — planned as a browser notification while a tab is open,
  see `UMSETZUNGSPLAN.md` WP 7.

---

## 11. Suggestions for additional features

> On the requirement "Propose any features I might have forgotten."

### High value, low effort

1. **File/folder import** — read in `.eml`, `.zip`, `.xml` via drag &
   drop. Already indispensable for development, enables offline
   operation and migration of old data. Falls out almost for free,
   because `MessageSource` is already abstracted.
2. **rDNS/PTR resolution** of source IPs with a cache. "192.0.2.45"
   becomes "mail-out.mailchimp.com" — the difference between raw data
   and insight.
3. **Detection of known services** — a curated list (Google Workspace,
   Microsoft 365, Mailchimp, SendGrid, Brevo, Postmark, …), matched via
   PTR and IP ranges. Immediately shows which legitimate service still
   isn't authenticating correctly.
4. **Export** — filtered view as CSV, charts as PNG.
5. **DNS check of your own domain** — live-query the current `_dmarc`
   TXT record, SPF, and DKIM selectors, and validate the syntax. Answers
   the question "is my configuration even correct?", which reports alone
   never reveal.

### Medium effort, high business value

6. **Diagnosis assistant** — explain in plain language *why* a failing
   source is failing (missing SPF / SPF not aligned / invalid DKIM
   signature / forwarding without SRS) and what to do about it.
7. **Policy maturity** — assess whether a move from `p=none` to
   `quarantine` or `reject` is safely possible: "98.7% of the last 30
   days pass DMARC; 2 unclassified sources remain."
8. **Alerting** — desktop notification on a pass-rate drop, a newly seen
   unknown sending source, or missing reports.
9. **Period comparison** — two periods side by side, differences
   highlighted.
10. **SPF lookup counter** — warning when exceeding the 10-DNS-lookup
    limit (RFC 7208), one of the most common silent failure causes.

### Operations and privacy

11. **Retention policy** — automatically delete reports older than *n*
    months. GDPR-relevant, since aggregate reports contain IP addresses.
12. **Backup/restore** — back up and restore the DB (`VACUUM INTO`).
13. **Scheduled background sync** — e.g. hourly, with status display.
14. **Headless CLI mode** — `dmarc-analyzer sync --headless` for
    cron/launchd, without starting the UI. Falls out almost for free from
    Clean Architecture, since the use cases are UI-independent.
15. **Multiple accounts and domains** in parallel, with a domain filter
    in all views.

### Later (interfaces are prepared)

16. DMARC Forensic Reports (RUF) — with a note on personally identifiable
    content.
17. TLS-RPT (RFC 8460).
18. BIMI readiness check.
19. MTA-STS policy check.

**Recommendation for v1:** plan in items 1–4 and 11; 5–8 as v1.1. The
rest are justified options, not a must.

---

## 12. Test strategy

> Requirement: "write tests."

### 12.1 Distribution

```
        ╱╲        few end-to-end tests (sync against an IMAP fake)
       ╱  ╲       integration tests (SQLite, parser with real fixtures)
      ╱____╲      many unit tests (domain, use cases)
```

### 12.2 Per layer

| Layer | Approach | Target coverage |
| --- | --- | --- |
| `domain` | Pure unit tests, table-driven, no mocks needed — the layer has no dependencies. | ≥ 90% |
| `app` | Use cases against **fakes** of the ports (hand-written, no mock framework). | ≥ 85% |
| `infra/dmarcxml` | **Golden-file tests** against real, anonymized reports from Google, Microsoft, Yahoo, Mail.ru, enterprise providers — every provider deviates from the RFC differently. Plus fuzzing on the XML parser. | ≥ 85% |
| `infra/sqlite` | Integration tests against a temporary file DB (not `:memory:`, so WAL and transactions are tested realistically). Migrations applied forward and repeatedly. | ≥ 80% |
| `infra/imap` | Tests against an in-process IMAP server (`go-imap` server component) with prepared messages. Checks UID logic and `UIDVALIDITY` changes in particular. | ≥ 70% |
| `web` | `net/http/httptest` with hand-written fakes of the app ports: status codes, redirects, security behavior (session/CSRF/`Host` check), every page rendered and parsed as HTML. JSON chart endpoints additionally checked for valid JSON and correct drill-down URLs. No pixel comparisons. | ≥ 80% |
| Browser (Chart.js) | **Deferred:** a `chromedp` smoke test (MIGRATIONSPLAN.md E-8) is meant to actually check the four charts in the browser (drawn, no console errors, one drill-down click works) — `httptest` can't see JavaScript. Still unwritten, see `MIGRATIONSPLAN.md` section 11. | — |

### 12.3 Special test cases

* Report with 0 records; report with 10,000 records.
* `UIDVALIDITY` change mid-sync → full rescan without duplicates.
* Duplicate-delivered identical mail → exactly one record.
* Corrupted gzip, zip with several files, zip bomb (size limit during
  unpacking).
* XML with unknown elements, missing `pct`, non-RFC-compliant enum
  values.
* XXE protection: external entities are rejected (Go's `encoding/xml`
  doesn't resolve them — pinned down by a test to keep it that way).
* Cancellation via `context.Cancel` mid-sync → consistent state.

### 12.4 Tools

* `go test -race` on every run — non-negotiable for a goroutine
  pipeline.
* `task cover` produces an HTML coverage report.
* Test fixtures live in `testdata/`; **all domains and IPs are
  anonymized** (`example.com`, RFC 5737 address ranges).

---

## 13. Taskfile

> Requirement: "generate a Taskfile with the common tasks."

| Task | Purpose |
| --- | --- |
| `task setup` | Install tools (`golangci-lint`), `go mod download`. |
| `task build` | Build the binary into `bin/` (`CGO_ENABLED=0`), embed the version via `-ldflags`. |
| `task run` | Start the program in development mode (opens the default browser). |
| `task test` | All tests with `-race`. |
| `task test:unit` | Fast tests only (without the `integration` build tag). |
| `task test:integration` | Integration tests (`integration` build tag). |
| `task cover` | Measure coverage and open it as HTML. |
| `task lint` | `golangci-lint run`. |
| `task fmt` | `gofmt -s -w` + `goimports`. |
| `task tidy` | `go mod tidy` and check for unused dependencies. |
| `task release:darwin` | Cross-compile macOS binaries (arm64, amd64), zip as a minimal `.app` bundle (`packaging/darwin/Info.plist.tmpl`, no more `fyne package` — see M5). |
| `task release:windows` | Cross-compile the Windows binary (amd64) without a console window (`-H windowsgui`), zip it. |
| `task release:linux` | Cross-compile Linux binaries (amd64, arm64), pack as `.tar.gz`. |
| `task release` | Run all three `release:*` tasks, produce checksums (`checksums.txt`). |
| `task clean` | Remove build artifacts. |
| `task check` | `fmt` + `lint` + `test` — the same thing CI runs. |

`task check` is the standard command before every commit. All release
tasks are plain `go build` cross-compiles with `CGO_ENABLED=0` (no more
platform-specific packaging tool needed, see M5 and
`docs/adr/0001-web-oberflaeche-statt-fyne.md`) — so they run on a single
development machine for all target platforms.

---

## 14. Documentation

**README.md** (German): what the program does, screenshots, installation
per platform, setting up the IMAP account (including a note on app
passwords), explanation of the metrics, storage locations of the DB and
configuration, privacy notice, development setup, license.

**CHANGELOG.md**: format per *Keep a Changelog*, versioning per *SemVer*.
Sections `Added` / `Changed` / `Fixed` / `Removed` / `Security`.
Maintained on every merge, not only at release time.

**docs/**: architecture overview with a diagram, ADRs (Architecture
Decision Records) for the load-bearing decisions — one short document
each for the SQLite choice, CGO freedom, keyring strategy, and chart
rendering. The value lies in still knowing, a year from now, *why*
something is the way it is.

---

## 15. Phase plan

| Phase | Content | Result |
| --- | --- | --- |
| **0 — Scaffolding** | `go mod init`, directory structure, `Taskfile.yml`, `.golangci.yml`, GitHub Actions workflow, README/CHANGELOG drafts, logging, path resolution. | `task check` runs green on an empty project. |
| **1 — Domain & parser** | Entities, value objects, invariants, ports. XML parser including gzip/zip unpacking. Fixtures from several providers. | Any report correctly turns into domain objects; golden tests green. |
| **2 — Persistence** | SQLite wiring, migrator, repositories, deduplication, query/aggregation functions. | Reports are saved and read back; integration tests green. |
| **3 — Mail & security** | IMAP adapter, keyring adapter, `SyncState`, incremental logic, error quarantine. | `SyncReports` fetches only new items from a real mailbox. |
| **4 — Application layer** | Use cases complete, statistics, export, CLI mode `sync --headless`. | The complete flow usable and testable without a UI. |
| **5 — UI scaffolding** | Window, navigation, theme, settings with connection test, report table, detail view, sync with progress. | A usable program for the core use case. |
| **6 — Analysis** | Dashboard, charts, sending-sources view, rDNS, service detection, filtering and grouping. | The visualization required by `FEATURES.md` is in place. |
| **7 — Polish & release** | File import, retention policy, notifications, background sync, accessibility, performance with large data volumes, packaging for three platforms, complete the docs. | Version 1.0.0. |

Every phase ends with a green `task check` and a maintained CHANGELOG
entry. Phases 1–4 are fully testable without a UI — that's the actual
payoff of the chosen architecture, and it keeps the feedback loop short.

---

## 16. Risks and countermeasures

| Risk | Impact | Countermeasure |
| --- | --- | --- |
| `go-imap/v2` is still under active development, the API can change | Rework effort on updates | Only the adapter behind `MessageSource` is affected; version pinned in `go.mod`. |
| Providers deliver RFC-deviating XML | Import fails | Tolerant parser, `Unknown` enums, error quarantine instead of aborting, raw-data archive for later re-reading. |
| `modernc.org/sqlite` slower than the CGO variant | Sluggish initial import | Batch inserts in transactions, indexes only after the bulk import, benchmarks in phase 2. |
| Fyne tables with very many rows | UI stutters | Keyset pagination in the data source, aggregations in SQL instead of Go. |
| Linux without a secret service | No keychain available | Encrypted file store as a fallback (see section 9). |
| Zip bomb in an attachment | Memory exhausted | Hard upper bound on unpacked size (e.g. 100 MB), `io.LimitReader`. |
| Aggregate reports contain IP addresses | GDPR obligations | Purely local processing, no cloud transfer, retention policy, note in the README. |
| Scope grows beyond v1 | Delay | Section 11 deliberately prioritizes; everything beyond v1 stays on hold until after 1.0.0. |

---

## 17. Open questions

| No. | Question | Suggestion if unanswered |
| --- | --- | --- |
| **O-1** | What is the module path / repo URL? | `github.com/Freie-Schule/dmarc-analyzer` |
| **O-2** | Should Go identifiers really stay English (comments and UI German)? See 1.3. | Yes, English identifiers. |
| **O-3** | Which license? | MIT |
| **O-4** | Should processed mail in the mailbox be marked or moved, or left untouched? | Untouched (`BODY.PEEK`), since several clients might read the same mailbox. Optionally toggleable. |
| **O-5** | Are there sample reports from a real mailbox for the fixtures? | Otherwise synthetic fixtures are generated from public RFC examples. |
| **O-6** | Are Windows and Linux actually target platforms, or is macOS enough? | All three build; testing is primarily on macOS. |
| **O-7** | Default retention period for reports? | 24 months, changeable in settings. |
| **O-8** | Should there be a headless/CLI mode (suggestion 14)? | Yes, in phase 4 — the effort is low, the value for automated runs high. |
