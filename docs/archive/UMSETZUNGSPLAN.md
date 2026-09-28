# Implementation progress plan — DMARC Analyzer

> As of: 2026-09-16. Reconciles `FEATURES.md` against the actual
> repository content, followed by the concrete work plan.
> `IMPLEMENTIERUNG.md` remains the architecture document (the *what* and
> *why*), this document is the work plan (the *in which order*).

---

## 1. Current state

The repository contains:

```
.gitignore          Go standard
FEATURES.md         requirements (1 line changed, uncommitted)
IMPLEMENTIERUNG.md  architecture plan, 713 lines
.idea/              IDE configuration
```

**There isn't a single Go file.** No `go.mod`, no `Taskfile.yml`, no
`README.md`, no `CHANGELOG.md`, no tests. Implementation progress against
`FEATURES.md` stands at **0%** — everything described below is new
work, none of it is follow-up.

Tools present locally and matching the plan: Go 1.27.1, Task 3.53.1,
golangci-lint 2.13.2.

---

## 2. Feature reconciliation

| # | Requirement from `FEATURES.md` | In the architecture plan | In the code | Gap |
| --- | --- | --- | --- | --- |
| F1 | Analyze and visualize DMARC data | Section 6, 10.3 | — | WP 1–2, WP 6 |
| F2 | UI for exploring and understanding the data | Section 10.1 | — | WP 5–6 |
| F3 | Various data formats, large data volumes | Section 7.1, 10.4 | — | **format matrix missing** (see 3.2) |
| F4 | Filter, sort, group | Section 10.1/10.3 | — | **only named, not specified** (see 3.3) |
| F5 | Charts and graphics | Section 10.3 | — | WP 6 |
| F6 | Fetch data from a mail account | Section 7 | — | WP 3 |
| F7 | Store and use credentials securely | Section 9 | — | WP 3 |
| F8 | Discover only new data in the mailbox | Section 7.1 | — | WP 3 |
| F9 | Propose additional features | Section 11 | — | done, selection open |
| N1 | Go binary with Fyne | Section 3 | — | WP 0 |
| N2 | Credentials stored securely | Section 9 | — | WP 3 |
| N3 | Taskfile with the usual tasks | Section 13 | — | WP 0 |
| N4 | Tests | Section 12 | — | in every WP |
| N5 | README.md | Section 14 | — | WP 0 (draft), WP 7 (final) |
| N6 | CHANGELOG.md | Section 14 | — | WP 0, then maintained per WP |
| N7–N10 | SOLID, DDD, Clean Architecture, Clean Code | Section 4.2 | — | structure from WP 0, checked via lint |
| N11 | Fyne best practices | Section 10.4 | — | WP 5 |
| **N12** | **User-friendly interface for easy navigation and operation** | **only implicit** | — | **newly added, see 3.1** |

---

## 3. What needs sharpening in the architecture plan

### 3.1 N12 — usability is new and so far only implicitly covered

This line was added to `FEATURES.md` after `IMPLEMENTIERUNG.md` was
written, and isn't treated there as a standalone requirement. Section 10
describes *which views* exist, but not what "user-friendly" is measured
against. Binding criteria for WP 5–7:

* **Guided first-run setup** — on first launch, a three-step wizard
  (account → test connection → first sync). No empty window with no
  guidance.
* **Empty states with a call to action** — every view with no data
  explains *why* it's empty and which button helps.
* **Technical terms explained** — DMARC vocabulary (disposition,
  alignment, policy) gets tooltips and a glossary. The target audience
  understands mail, not RFC 7489.
* **Plain-language error messages** — "Login failed — if two-factor
  authentication is enabled, an app password is required." instead of
  `imap: NO [AUTHENTICATIONFAILED]`. The original technical text is
  available expanded.
* **Keyboard operation** — full tab order, `Cmd/Ctrl+R` sync,
  `Cmd/Ctrl+F` filter, `Esc` closes dialogs.
* **No blocking UI** — every action over 300ms shows progress and can be
  cancelled.
* **Accessibility** — pass/fail never conveyed by color alone, minimum
  contrast 4.5:1, font size follows the system setting.

### 3.2 F3 — "various data formats" needs a binding list

The plan names `.gz`, `.zip`, and plain `.xml`. Fixed for WP 1:

| Format | Handling |
| --- | --- |
| `report.xml` | parse directly |
| `report.xml.gz` | `compress/gzip`, 100 MB size limit |
| `report.zip` | `archive/zip`, **all** contained XML files, limit per entry and in total |
| `.eml` file | split MIME like an IMAP message (file import, suggestion 11.1) |
| base64 inline body with no attachment | detect and decode — occurs with some providers |
| unknown | quarantine in `failed_imports`, the run continues |

### 3.3 F4 — filter/sort/group belongs in SQL, not the UI

So "efficiently handle large data volumes" (F3) doesn't become a lie:
`ReportQuery` gets filter fields (time range, domain, org, source IP,
disposition, pass/fail) in WP 2, a sort key with direction, a grouping
dimension, and keyset pagination. The UI table in WP 5 lazy-loads only
visible pages. No `SELECT *` into RAM, no sorting in Go.

### 3.4 Module path and keyring service are proposed incorrectly

`IMPLEMENTIERUNG.md` proposes `github.com/Freie-Schule/dmarc-analyzer`
and the keyring service `de.freie-schule.dmarc-analyzer`. The repository
lives under `pmoscode/dmarc-analyzer`. **Decision:**

* Module path: `github.com/pmoscode/dmarc-analyzer`
* Keyring service: `de.pmoscode.dmarc-analyzer`

The keyring service can only be changed later with migration effort —
that has to be decided before the first line of code, not after.

### 3.5 Open questions: proposed values apply until challenged

O-2 through O-8 from `IMPLEMENTIERUNG.md` are implemented with the
answers proposed there (English identifiers, MIT license, `BODY.PEEK`
with no marking, 24-month retention, CLI mode yes, all three platforms
build). Only O-5 stays genuinely open: **real sample reports from the
mailbox would be valuable for WP 1.** Without them, synthetic fixtures
are generated from the RFC examples — that covers provider quirks less
well.

---

## 4. Work packages

Every WP ends with a green `task check` and a CHANGELOG entry.
WPs 0–4 are fully testable without a UI.

### WP 0 — Scaffolding

**Goal:** `task check` runs green on an empty but fully configured project.

- [x] `go mod init github.com/pmoscode/dmarc-analyzer`
- [x] Create the directory tree per `IMPLEMENTIERUNG.md` section 5
- [x] Researched dependencies and **pinned to concrete versions** (Fyne,
      go-imap/v2, go-message, modernc.org/sqlite, go-keyring,
      go-chart/v2, testify) — details and one deviation from the
      original idea in `docs/DEPENDENCIES.md`: only `testify` is
      already in `go.mod`, because it's actually imported in tests from
      WP 0 on. The other five are documented there with a target
      version for their respective phase instead of being forced into
      `go.mod` unused — otherwise the next `task tidy` removes them
      again, and `go.mod` would lie about the actual state.
- [x] `Taskfile.yml` with the 16 tasks from section 13
- [x] `.golangci.yml`: govet, staticcheck, errcheck, revive, gosec,
      ineffassign, gocritic — **`misspell` deliberately left out**: the
      German comments (project convention, see 1.3) constantly trigger
      false positives with an English dictionary (`Konfiguration` →
      `Configuration`), explained in `.golangci.yml`.
- [x] `.github/workflows/ci.yml`: `task check` + `task build` on
      ubuntu/macos/windows, plus a check for gofmt/goimports drift
- [x] `internal/platform/logging` (slog, text format locally, JSON via an option)
- [x] `internal/platform/paths` (DB, log, config path, platform-appropriate, with tests)
- [x] `cmd/dmarc-analyzer/main.go` — starts, logs the version, exits, with tests
- [x] `README.md` and `CHANGELOG.md` as a draft, `LICENSE` (MIT)

**Done when:** `task build` produces a runnable binary, `task check` is
green, CI is green on all three platforms. — Reached locally (`task
check`, `task build` both green, coverage `platform/*` 83%). CI green
on all three platforms can only be verified after the first push/PR.

### WP 1 — Domain and parser

**Goal:** Any report correctly turns into domain objects.

- [x] `internal/domain/report`: entities, value objects, constructors
      with invariants (section 6.1/6.2) — including the `Repository`
      port (section 6.4), pulled forward from WP 2 because the
      interface belongs next to the aggregate (the implementation stays
      in WP 2)
- [x] Unknown enum values → `Unknown`, never discarded — for
      Disposition, Policy, AlignmentMode, and AuthResultValue, backed
      by a golden-file test
- [x] `internal/domain/sync`: `State`, `ReportParser` — **`MessageSource`
      deliberately not yet defined**, see the comment in `ports.go`:
      the port depends on `account.MailAccount`, which only appears in
      WP 3. Introducing it now with a placeholder account type would
      fake a dependency that doesn't exist yet. For the same reason the
      type is called `sync.State`, not `sync.SyncState` (avoids the
      stutter `sync.SyncState`) — the same consequence was applied to
      `report.Key`/`Query`/`Page`/`Repository` (instead of `ReportKey`
      etc.), `ReportID` stayed an exception (field collision with
      `AggregateReport.ID`, see the comment in the code).
- [x] `internal/infra/dmarcxml`: XML parser, tolerant of provider
      deviations (casing, missing `pct`, missing `adkim`/`aspf` with
      the RFC default `r`, unknown enum values)
- [x] Unpacker for the format matrix from 3.2 (`.xml`, `.xml.gz`, `.zip`
      with multiple XML files), 100 MB size limit against zip/gzip bombs
- [x] Fixtures in `testdata/reports/` — **synthetic from RFC 7489
      examples** (E-4 stayed unanswered, the fallback from section 6
      applied), five provider styles (RFC example, Google, Microsoft,
      multi-report zip, three edge cases), `example.com`/RFC 5737
      addresses
- [x] Golden-file tests per provider style, fuzz test (`FuzzParse`, 30s
      / 2.26M runs verified locally with no findings)
- [x] XXE test — with a correction to the original assumption:
      `encoding/xml` doesn't silently resolve the external entity, it
      rejects the whole document. The test now checks exactly that
      (an error, not a success with the entity ignored).

**Done when:** `domain` coverage ≥ 90%, `dmarcxml` ≥ 85%, fuzzing runs
60s with no findings. — Reached: `domain/report` 100%,
`infra/dmarcxml` 90.7%, fuzz 30s with no findings (locally; 60s is to
be caught up on in CI or before release). The two zip/gzip-bomb tests
are marked with `testing.Short()` so `task test:unit` stays fast (~1.5s
instead of ~7s).

### WP 2 — Persistence

**Goal:** Reports are saved, deduplicated, and queried performantly.

- [x] SQLite connection with PRAGMAs (WAL, foreign_keys, busy_timeout,
      synchronous) — as DSN parameters on open, not as separate PRAGMA
      statements (`_journal_mode`/`_foreign_keys`/`_busy_timeout`/
      `_synchronous`, modernc.org/sqlite's own short form)
- [x] A custom migrator (~150 lines) + embedded `migrations/*.sql`
      (schema per section 8.1, extended with `report_errors` plus
      `mailbox`/`filename`/`org_extra_contact_info` on `reports` — the
      section 8.1 table was marked as an "excerpt", that was the gap
      versus the complete domain model from WP 1)
- [x] `reportrepo.go` with batch insert (prepared statements per table,
      reused across all records of a report) in one transaction
- [x] Deduplication via UNIQUE `(org_name, report_id, date_begin)`,
      detected as `ErrDuplicateReport` via `sqlite.Error.Code()` (not
      string-matching on the error message)
- [x] `Query` complete per 3.3: filters (time-range overlap, domain,
      org, source IP, disposition via `EXISTS` subqueries), sorting,
      keyset pagination via SQLite row-value comparisons.
      **Clarification on `GroupBy`:** acts as an additional primary sort
      key (same group stands together), not as SQL `GROUP BY` with
      aggregation — `Query` still returns individual `AggregateReport`
      values per the port contract, not metrics. The adapter rejects
      `GroupBySourceIP` with an error (source IP is a record property,
      not a report property). See the comments in
      `domain/report/repository.go`.
- [ ] Aggregations for the metrics from section 10.2 **in SQL** —
      **deliberately not in WP 2.** Those are application-side
      questions (pass rate under the alignment definition, trend vs.
      the previous period), not a pure persistence task — they belong
      to the statistics use case in WP 4, which gets its own, tailored
      repository methods for this instead of overloading
      `Query`/`GroupBy`.
- [x] `raw_reports` and `failed_imports` — **schema only** (tables +
      indexes exist). Actually writing to them (raw-data archive, error
      quarantine during sync) is part of the SyncReports use case in
      WP 3/4, not the pure persistence layer.
- [x] Integration tests against a temporary file DB (never `:memory:`),
      migration applied twice in `TestMigrate_IsIdempotent`
- [x] Benchmark: import and query 100,000 records — **as a regular test
      skipped via `testing.Short()`** instead of `go test -bench`: the
      scenario is a one-shot flow (build up a data volume, measure one
      realistic query), not a micro-benchmark loop. Results below.

**A real performance bug found and fixed along the way:** the first
version of the batch loading functions for records/reasons/auth results
built an `IN (?, ?, ..., ?)` clause with one placeholder per record —
at 10,000 records, just preparing these statements alone took so long
that `FindByID` needed over 3 seconds. Indexes were also missing on
`record_reasons.record_id`, `auth_results_dkim.record_id`,
`auth_results_spf.record_id`, and `report_errors.report_id` (the schema
was still moving before the first release, no `0002` migration
detour needed). Both fixed: indexes added, the three loading functions
switched from an IN clause to `JOIN records ON ... WHERE
records.report_id = ?`.

**Done when:** double-importing the same mail produces exactly one
record — reached (`TestSave_DuplicateKey_ReturnsErrDuplicateReport`). A
query over 100,000 records under 100ms — reached for the actual target
path, the report table: measured locally at **~7-8ms** for a sorted,
filtered page over 1,001 reports with 110,000 records total. Loading a
single, unrealistically large report with 10,000 records (`FindByID`,
detail view) comes in at **~460ms** — noticeably over 100ms, but an
edge case (real DMARC reports rarely have more than low hundreds of
records) and acceptable for a one-off detail view; noted as a possible
future optimization point, not a blocker for WP 2.

### WP 3 — Mail and security

**Goal:** Only new items are fetched from a real mailbox.

- [x] IMAP adapter (`internal/infra/imap.Adapter`): TLS enforced when
      `MailAccount.UseTLS` (context-aware dial via `tls.Dialer`/
      `net.Dialer`, never `InsecureSkipVerify`), `BODY.PEEK[]` **and**
      `EXAMINE` instead of `SELECT` (read-only, doubly secured),
      UID-based, streaming `iter.Seq2` iterator via
      `FetchMessageData.Collect()` per message (not the whole result
      set at once)
- [x] `UIDVALIDITY` change → full rescan (`startUID = 1`,
      `baseline.LastUID = 0`), the UNIQUE index from WP 2 catches
      duplicates — verified with an in-process server
      (`TestFetchNew_UIDValidityChange_TriggersFullRescan`)
- [x] Persist `SyncState` (`sync.State`) after every message —
      **clarification:** `FetchNew` itself persists nothing, but
      returns a baseline state (mainly the current `UIDValidity`) plus
      the UID of each message; the caller (the SyncReports use case, WP
      4) persists `LastUID` after every successfully processed message.
      See the comment on `sync.MessageSource` in `ports.go`.
- [x] Backoff on transient errors (3 attempts, exponential) — **only for
      connection setup**, not for login: a wrong password doesn't
      become right by retrying, and repeated failed attempts can lock
      accounts at the provider. Verified with a real transient error
      (`TestConnect_RetriesTransientDialFailure`: the server only
      starts during the backoff wait between attempt 1 and 2).
- [x] Keyring adapter (`internal/infra/keyring.OSStore`), service
      `de.pmoscode.dmarc-analyzer`
- [x] Type `Secret` (`internal/domain/account`) with masking
      `String()`/`MarshalJSON()` + tests. **A real leak found and fixed
      here:** `%#v` uses `fmt.GoStringer`, not `fmt.Stringer` — without
      a custom `GoString()` method, `%#v` would have shown the raw
      secret bytes hex-encoded, even though `%v`/`%+v` already masked
      correctly. The first test draft wouldn't have caught this (an
      ASCII substring search doesn't detect hex encoding) — the test
      was tightened accordingly. See AGENTS.md, "Masked types" section.
- [x] Linux fallback (`internal/infra/keyring.FileStore`): AES-256-GCM
      (standard library `crypto/aes`/`crypto/cipher`), a key derived via
      `scrypt` (newly pinned: `golang.org/x/crypto`, see
      `docs/DEPENDENCIES.md`) from a master passphrase + a persisted
      salt. `keyring.IsAvailable()` checks the OS keychain via a canary
      value instead of guessing from a particular error class — the
      decision of *which* store to use is made by the composition root
      (WP 4/5), not the adapter itself.
- [~] Deleting an account removes metadata **and** the keyring entry —
      **only the keyring half is WP 3**: `CredentialStore.Delete` is
      implemented and tested (idempotent). The metadata half needs
      `account.Repository` against SQLite (`accountrepo.go`), which —
      for the same reason as `MessageSource` in WP 1/2 — deliberately
      only comes in WP 4, once account management
      (`internal/app/manageaccount`) brings both halves together. The
      `account.Repository` port is already defined.
- [x] Tests against an in-process IMAP server (`imapmemserver`, go-imap's
      own test server component), 11 tests including UID logic, context
      cancellation (before and during iteration), PEEK/no-\Seen-flag.
      **A bug found in the beta library:**
      `imapmemserver.User.Append` with `nil` options dereferences
      without checking and panics — worked around with
      `&imap.AppendOptions{}`, documented in the test.

**Done when:** a second sync run right after the first fetches zero
messages — reached, `TestFetchNew_SecondSync_ReturnsNoNewMessages`
reproduces exactly that. No test run contains a password in logs or
error text — the test-server logs were checked, `Secret` prevents it
structurally; the one known limit: the copy of the password passed to
`imapclient.Login` as a `string` can no longer be actively overwritten
(Go strings are immutable), see the comment on `Secret.Zero()`.

### WP 4 — Application layer

**Goal:** The complete flow is usable without a UI.

- [x] Use cases `syncreports`, `queryreports`, `statistics`,
      `manageaccount`, `exportdata` — plus `importfiles` (see the next
      point) and, as a byproduct, `internal/infra/mailmime` (MIME
      splitting, deliberately deferred from WP 3) and three new SQLite
      repositories (`accountrepo.go`, `syncstaterepo.go`,
      `failedimportrepo.go`), also deliberately moved here from WP 1–3,
      to where the actual consumers appear. `statistics` gained an
      `analysis` port for this (statistics types, `Repository.Compute`)
      — the SQL aggregation deferred in WP 2 ("aggregations for the
      metrics") lands here, tailored rather than as an extension of
      `report.Query`.
- [x] Fetch/parse pipeline concurrent, writing serial, `-race` clean —
      `syncreports.runPipeline`: one fetcher goroutine, a parser worker
      pool (configurable size), a single writer. **A design detail
      beyond the letter of the plan:** since workers can finish out of
      order, a `progressTracker` tracks the gapless progress boundary
      instead of simply adopting the highest UID seen — otherwise
      `LastUID` could skip a not-yet-saved message on a crash. Backed
      by targeted tests for exactly this out-of-order case.
- [x] Progress and result reports (new/skipped/failed) —
      `Result{New, Skipped, Failed, Errors}`, failed attachments land in
      `failed_imports` (a new `FailedImportRepository` port).
- [x] File import (suggestion 11.1) — **deliberately NOT** as a second
      `MessageSource`: since WP 3 the port requires `account.MailAccount`
      + `Secret` for `Connect`, which a local file import doesn't have.
      A dedicated use case `internal/app/importfiles`, which shares
      `MessageDecoder` and `ReportParser` with `syncreports` (the same
      MIME splitting for IMAP as for `.eml` files) and uses the same
      dedup logic — extracted for this as a domain-side
      `report.SaveIfNew(ctx, repo, r)` (see below), instead of
      duplicated in both use cases.
- [x] CLI: `dmarc-analyzer sync --headless`, `import <path>`, `stats` —
      plus `account add|list|test|delete` (not literally required, but
      without any way to create an account, `sync` wouldn't be usable
      from the CLI at all; see the discussion below under "done when").
      The composition root (`cmd/dmarc-analyzer/wire.go`) wires up
      adapters **only for the subcommand actually invoked** —
      `stats`/`import` don't build a credential store and thus never
      touch the real OS keychain, that happens exclusively for
      `sync`/`account`.
- [x] All use cases tested against hand-written fakes, coverage ≥ 85% —
      reached: `syncreports` 92.1%, `manageaccount` 91.7%, `statistics`
      90.9%, `importfiles` 86.2%, `exportdata` 87.8%, `queryreports`
      100%.
- [x] Cancellation via `context.Cancel` leaves a consistent state (test)
      — `TestSyncAccount_ContextCancelledMidSync_LeavesConsistentState`:
      `LastUID` never skips a message that wasn't actually processed,
      reports counted as "new" are always actually saved.

**Two real bugs found and fixed along the way** (discovered by failing
tests, not just claimed):
1. `ErrDuplicateReport` sat in `internal/infra/sqlite` — `syncreports`
   would have had to import it, a DIP violation (the app layer may only
   depend on domain ports, AGENTS.md). Moved to `report.ErrDuplicate`
   in the domain port, `sqlite.ReportRepository.Save` now returns it
   (wrapped) instead of its own sentinel.
2. A previously unstated contract on `account.CredentialStore.Store`:
   the real adapters (`OSStore`, `FileStore`) synchronously turn the
   secret bytes into their own copy (string conversion or encryption),
   and a caller may overwrite the passed `Secret` with `Zero()`
   immediately afterward. A naive test fake that just stores it
   shallowly would get wiped along with it by the same `Zero()` call
   (shared backing array) — actually occurred in the `account add` CLI
   test. The contract is now documented on the port, all three fake
   `CredentialStore` implementations in the repo now copy defensively
   (`account.NewSecret(s.Expose())`).

**Done when:** a complete import from a mailbox runs through via the
CLI, including a metrics printout. — Reached, with one honest
limitation: verified end-to-end via the real, built binary for **file
import** (`dmarc-analyzer import` with three real provider-style
fixtures — RFC 7489 example, Google gzip, Microsoft zip —, then
`dmarc-analyzer stats` with hand-checked values, duplicate detection on
re-import). **Not** tested end-to-end with the real binary against a
real/simulated mailbox — that would require `account add`, which would
touch this machine's real OS keychain, and I didn't want to trigger that
without asking. The IMAP path itself is instead doubly secured: WP 3
tests the adapter against a real in-process IMAP server, WP 4 tests
`syncreports.SyncAccount` (the use case that calls it) extensively
against fakes, including the exact "second run fetches nothing"
criterion.

### WP 5 — UI scaffolding

**Goal:** A usable program for the core use case.

- [x] Main window, side navigation, its own theme (light and dark)
- [x] `internal/ui/i18n` — **all** text centralized, no string in widget code
- [x] Settings: create an account, test the connection with clear feedback
- [x] First-run wizard per 3.1
- [x] Report table with a lazy data source via `ReportQuery`
- [x] Detail view of a report
- [x] Sync button with progress and cancel, I/O never on the UI thread,
      returning via `fyne.Do()`
- [x] Empty states and plain-language error messages per 3.1
- [x] Tests with `fyne.io/fyne/v2/test`: layout, navigation, form validation

**Done when:** a user sets up an account with no documentation and sees
their reports. ✅ Reached.

**Deviations / findings:**
- The custom theme deliberately limits itself to an accent color
  (`appTheme.Color` overrides only `ColorNamePrimary`); light/dark
  contrast and switching are handled unchanged by Fyne's `ThemeVariant`
  system, since that's already verified accessible — a custom contrast
  system here would only have added risk with no benefit.
- Real bug found and fixed: `settings.View.refreshContent()` incorrectly
  wrote the new list/empty-state content to
  `v.container.Objects[len(v.container.Objects)-1]`. In
  `container.NewBorder(top, nil, nil, nil, center)`, the variadic
  `objects` argument (here `center`) always ends up at index 0 — the
  top/bottom/left/right objects are appended only after that. This
  meant every `Reload()` overwrote the header (title "Accounts" + add
  button) with the list/empty state instead of the other way around —
  this only became visible through the failing shell navigation test
  `TestShell_SelectNav_SwitchesToSettings`. `reports.View` already had
  the same pattern right (index 0). Fix: both branches now write to
  `Objects[0]`.
- The Fyne test driver runs `fyne.Do()` synchronously on the calling
  goroutine (unlike the real driver) — without the injectable
  `runBackground` field (default: a real goroutine, replaced
  synchronously in tests), tests under `-race` would have reported real
  data races. See AGENTS.md.

### WP 6 — Analysis and visualization

**Goal:** The visualization required by `FEATURES.md` is in place.

- [x] Dashboard with metric tiles including trend vs. the previous period
- [x] `ChartRenderer` port + go-chart adapter
- [x] Time series, top-10 bars, disposition donut, source × day heatmap
- [x] Sending-sources view, aggregated by source IP
- [x] rDNS/PTR resolution with a cache (suggestion 11.2)
- [x] Detection of known services via PTR and IP ranges (suggestion 11.3)
- [x] Filter and grouping bar, acts on all views
- [x] Glossary and tooltips for DMARC terms
- [x] Export: filtered view as CSV, chart as PNG (suggestion 11.4)

**Done when:** from the dashboard, it's clear without an intermediate
step which sending source is failing and how many messages are
affected. ✅ Reached.

**Deviations / findings:**
- `analysis.Repository` (WP 2/4) extended with `DailyVolumes`,
  `TopSources`, `Heatmap` instead of a new port — stays the one place
  for SQL-aggregated read access to metrics, consistent with `Compute`.
  A report is assigned to the day of `date_begin` for the time
  series/heatmap, not spread proportionally — DMARC aggregate reports
  are, per RFC 7489, practically always single-day periods, and a
  proportional split wouldn't have delivered meaningful business value
  relative to the effort.
- New domain port `domain/sources` (package name `sources`, like
  `internal/ui/sources` — different import paths, imported in the UI
  code under the alias `domainsources`, following the same pattern as
  `domainsync`) for the view aggregated by source IP, plus an
  `Enricher` port (PTR + service detection). SQL aggregation
  (repository) and network enrichment (enricher) are deliberately
  separate ports and run in different layers (repository in
  `infra/sqlite`, the enrich loop in `app/sourcestats`) — two
  fundamentally different kinds of I/O that a shared adapter would have
  conflated.
- `internal/infra/sourceinfo.Enricher`: only hostname-based service
  detection (PTR suffix), deliberately **without** additional IP-range
  detection despite the `FEATURES.md` wording ("via PTR and IP
  ranges") — hardcoded CIDR lists of big providers would go stale
  unnoticed after a short time, and without a maintenance process for
  that, it's worse than no information at all. The cache is an
  unbounded in-process `sync.Map` (no TTL) — sufficient for a desktop
  application with short runtimes.
- go-chart/v2 has no heatmap chart type. The heatmap is therefore drawn
  not via go-chart, but directly with `image/draw` (rectangles) plus
  go-chart's own `drawing.RasterGraphicContext` for text (axis labels)
  — saves an extra font-rendering dependency just for this one chart.
- `ChartRenderer` returns an empty white image for empty input data
  instead of an error — the calling `dashboard.View` shows an empty
  state instead of a chart in that case anyway; an error would have
  been unnecessarily strict here.
- Chart labels (e.g. "Passed"/"Failed", disposition names) are kept as
  literal German strings directly in `internal/infra/charts`, not
  imported from `internal/ui/i18n` — `i18n` is a UI-layer dependency
  that `infra` isn't allowed to import per the dependency direction. A
  small, deliberately accepted text duplication.
- "Tooltips" (a checklist item) aren't a hover tooltip — Fyne v2.8 has
  no built-in tooltip mechanism (checked: no match in the entire
  module). Substitute: a small "?" button next to individual dashboard
  tiles, or in the main window header, opens the same glossary text as
  a dialog (`internal/ui/glossary`).
- The "grouping bar" wasn't added to the shared `FilterBar`:
  `analysis.Query` and `sources.Query` don't know `GroupBy` (overview/
  sending sources are already aggregated, an additional grouping
  wouldn't make sense there). Instead, only `reports.View` got its own
  grouping `Select` (none/domain/organization), which finally connects
  `report.Query.GroupBy` — present since WP 2 — to the UI.
- Export exports the currently loaded page(s) (`reports.View`/
  `sources.View`), not necessarily every record matching the filter
  overall — pre-loading every page just for export would itself be a
  performance trap on large datasets and would contradict the lazy
  data source deliberately chosen in WP 2/5.
- **Real bug found and fixed** (construction order, not a test
  artifact): `widget.Select.SetSelected()` fires `OnChanged`
  synchronously, even from the constructor. The new grouping `Select`
  in `reports.View` called `SetSelected()` before `v.container` was
  assigned — the synchronously fired `OnChanged` handler accessed the
  still-nil `v.container` via `Reload()`/`setCenter()` and panicked
  right at the `NewView()` call. Uncovered by a simple construction
  test (`NewView()` + `SetContent`), not by complex fixtures. Fix:
  build the `Select` first without `OnChanged`, assign `OnChanged` only
  after the rest of the widget is done — see AGENTS.md.
- **A WP 5 gap closed:** there was previously no way to actually launch
  the graphical program — `ui.BuildMainWindow` existed, but `main.go`
  never called it (only `run()`/subcommands). Now `dmarc-analyzer` with
  no arguments starts the GUI (`cmd_gui.go`, `fyneapp.New()` +
  `window.ShowAndRun()`); `--help`/`-h` still only shows the
  command-line help. Deliberately **not** wired into `run()`, since
  `run()` is called directly with empty arguments by `cmd_*_test.go`
  (`TestRun_NoArgs_PrintsUsageWithoutError`) and must never open a real
  window there — the GUI startup decision therefore lives exclusively
  in `main()`, which isn't unit-tested. `"gui"` was added to
  `credentialAwareCommands`, since both account management and the sync
  button in the main window need credentials. An actual GUI launch in a
  desktop environment was **not** verified in this session (no display
  available in the development container) — only `--help` and all
  existing subcommands were checked against the built binary; the Fyne
  widget layer itself is thoroughly tested via `fyne.io/fyne/v2/test`
  (headless driver).

**Follow-up addition (after WP 6 finished):** a folder picker for the
account form — DMARC reports don't necessarily land in the root mailbox
(INBOX), but can, for example, be filed into a subfolder via a mail
rule. The mailbox field itself always technically supported an arbitrary
folder path (free text, passed through unchanged to IMAP
SELECT/EXAMINE), but without help the user had to blindly guess the
exact path including the server-dependent separator (e.g. `/` for
Dovecot, `.` for Courier).
- A new, optional port `sync.MailboxLister` (`domain/sync/ports.go`) —
  deliberately separate from `MessageSource` instead of another method
  there, since not every conceivable source can/must list mailboxes;
  callers check via a type assertion. `imap.Adapter` additionally
  implements it via `IMAP LIST`, filtered to actually selectable
  mailboxes (the `\Noselect` attribute excluded).
- `manageaccount.UseCase.ListMailboxes` connects on a trial basis (like
  `TestConnection`, without syncing) and lists them.
- `settings.AccountForm.mailbox` is now a `widget.SelectEntry` instead
  of `widget.Entry` — still freely typable (e.g. for servers where LIST
  doesn't return the right thing for some reason), additionally
  fillable with dropdown options via the new "list folders" button
  (uses the credentials currently entered in the form, without the
  account needing to already be saved — this works both when creating
  one in settings and in the first-run wizard).
- A small test trap caused by the switch to `SelectEntry`, found and
  fixed: `uitest.FindEntries` doesn't recognize `*widget.SelectEntry`
  (a type assertion on the concrete type `*widget.Entry`, `SelectEntry`
  only embeds `Entry`) — an index-based test helper in the first-run
  wizard (`fillValidAccountForm`) had to be recounted. See AGENTS.md.

### WP 7 — Polish and release 1.0.0

> **Inserted before this WP:** `MIGRATIONSPLAN.md` (the move from the
> original Fyne desktop UI to a web UI embedded in the program,
> milestones M0–M6). Status here: M0–M5 implemented, see section 10
> there. The items below are adjusted accordingly (desktop notification
> → browser notification, packaging without `fyne package`) or already
> fully or partly done via M5.

- [x] ~~Retention policy, default 24 months (suggestion 11.11)~~ —
  a settings page (field "report retention (months)", default 24, 0 =
  unlimited), persisted via `internal/infra/config.Store` (a JSON file,
  `internal/domain/settings`), applied via `internal/app/retention` and
  automatically in the background via `internal/app/retentionjob` (once
  at startup, then every 24h). Deletion via
  `report.Pruner`/`ReportRepository.DeleteOlderThan` (SQLite `DELETE
  ... WHERE date_end < ?`, cascading via existing foreign keys).
- [x] ~~Scheduled background sync + **browser notification while a tab
  is open**~~ (instead of the originally planned
  `fyne.App.SendNotification` desktop notification — since the move to
  the web UI there's no more desktop app window that could trigger a
  native notification; the notification therefore runs via the browser
  Notifications API and requires an open tab) — the sync interval
  (minutes, 0 = off, default 60) also on the settings page, the
  scheduled sync via `internal/app/syncscheduler` (checks every minute,
  based on the current setting, whether a run is due, starts it via the
  existing `syncjob.Runner`). Notification in `static/app.js`: a button
  on the settings page requests `Notification.requestPermission()` on a
  user click, the existing SSE script shows a browser notification on
  the transition from "running" to "done"/"cancelled"/"failed" if
  permission was granted.
- [ ] Backup/restore via `VACUUM INTO` (suggestion 11.12)
- [ ] Accessibility pass per the criteria from 3.1 (see also
  `MIGRATIONSPLAN.md` M6)
- [ ] Load test with several years of report data
- [x] ~~Packaging for macOS/Windows/Linux, `task release` with
  checksums~~ — **implemented in M5** (`Taskfile.yml`:
  `release:darwin`/`release:windows`/`release:linux`/`release`, plain
  `CGO_ENABLED=0` cross-compiles without `fyne package`, see
  `MIGRATIONSPLAN.md`). Still open for a real 1.0.0 release: macOS
  signing/notarization (currently unsigned, see README "Installation"),
  possibly additional distribution channels (Homebrew, winget, …) —
  deliberately not part of M5.
- [ ] Final README with screenshots and a privacy notice — the text
  parts were already updated in M5 (start/browser behavior/quit/
  `--kein-browser`/security model), **screenshots still missing**. ADRs
  in `docs/adr/` already created in M5 (0001 web UI instead of Fyne,
  0002 Chart.js instead of a `ChartRenderer` port).
- [ ] CHANGELOG entry `1.0.0`, set the tag

---

## 5. Order and dependencies

```
WP 0 ──▶ WP 1 ──▶ WP 2 ──▶ WP 3 ──▶ WP 4 ──▶ WP 5 ──▶ WP 6 ──▶ WP 7
                    │                          │
                    └──── WP 4 needs 2 + 3 ────┘
```

WP 1 and WP 2 can partly run in parallel (the parser against fixtures,
the schema against hand-built data), WP 3 needs both. WP 4 should be
fully tested before WP 5 — otherwise UI bugs and logic bugs become
indistinguishable.

---

## 6. To decide before the first commit

| No. | Question | Default if no answer comes | Status |
| --- | --- | --- | --- |
| E-1 | Module path `github.com/pmoscode/dmarc-analyzer`? | yes | **implemented** (WP 0) |
| E-2 | Keyring service `de.pmoscode.dmarc-analyzer`? | yes | **implemented** (WP 0, documented in the README; adapter follows in WP 3) |
| E-3 | MIT license? | yes | **implemented** (WP 0, `LICENSE`) |
| E-4 | Real sample reports available for the fixtures? | no → synthetic from RFC examples | open — clarify before WP 1 |
| E-5 | v1 scope: WPs 0–7 as above, or cut after WP 5? | full scope through WP 7 | open (the default applies for now) |

E-1 through E-3 can only be changed later with migration effort — they're
now fixed as of WP 0.
