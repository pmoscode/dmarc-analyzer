# AGENTS.md

Guide for AI coding agents in this repository. Human contributors find the
same information in more detail in
[`docs/architecture.md`](docs/architecture.md) (architecture) and
[`docs/features/`](docs/features/) (per-domain feature docs).
`docs/archive/` contains the planning documents from the earlier
desktop era — historical context, not the current state (see
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`).

## What this is about

DMARC Analyzer: a Go program that runs as a Docker container, fetches DMARC
aggregate reports from an IMAP mailbox, stores them in SQLite, and evaluates
them via an embedded web UI. Fully configured through environment variables
(`internal/infra/envconfig`, see `docs/features/deployment.md`), access
protected via OIDC against an Authentik IdP (`docs/features/auth.md`).
Exactly one IMAP account per container, no account CRUD in the UI.

## Language rule — important, breaks CI otherwise

- **Comments, doc comments, log messages, docs: English.**
- **Anything rendered in the browser: German.** HTML template text,
  JS-injected UI strings (labels, tooltips, chart legends, browser
  notifications), `http.Error(...)` response bodies, page titles, nav
  labels, flash/redirect messages, and `internal/web/glossary` tooltip
  text are all German. `internal/web/templates/layout.html` sets
  `lang="de"` accordingly.
- **Go identifiers (packages, types, functions, fields): English**
  regardless of the above — a German UI string lives in a value, never in
  an identifier.
- **Error messages: it depends on whether the message ever reaches the
  browser verbatim.** Most don't: `internal/web`'s `serverError` helper
  (`handlers.go`) logs the real `err` via `s.logger.Error(...)` (English)
  and replaces it with a fixed, already-German response text — so
  `internal/app`/`internal/infra`/`internal/domain` error messages stay
  English like the rest of those layers, *unless* a specific call chain
  is wired to display `err.Error()` to the user directly. Right now there
  is exactly one such chain, and it is German for that reason:
  `internal/web/handlers_sync.go`'s `newSSEState` puts
  `syncjob.State.Err.Error()` into the `/ereignisse` SSE stream, and
  `static/app.js` shows it verbatim in the sync-failed status line and
  browser notification. That pulls in `internal/app/syncreports.UseCase.
  SyncAccount`'s five top-level wrapped errors and what they in turn wrap
  in `internal/infra/sqlite/accountrepo.go` (`FindAll`/`FindByID`),
  `internal/infra/imap/{adapter,dial,backoff,fetch}.go` (`Connect`/
  `FetchNew`'s direct return only), and `internal/infra/sqlite/
  syncstaterepo.go` (`Load`/`Save`). Everything else in those same
  files — comments, per-report/per-message errors that only ever get
  counted via `Result.Failed` (never shown as text), unrelated errors —
  stays English. Before wiring a new error path into something the UI
  displays verbatim, either author that specific message in German or
  replace it with a fixed, translated message at the `internal/web`
  boundary (the `serverError` pattern) — don't assume an error's text
  will automatically show up in the right language just because it's
  German (or English) at its source.
- `misspell` stays disabled in the linter (`.golangci.yml`) — a
  deliberately mixed-language codebase (English internals, German UI)
  defeats an English dictionary check.

## Commands

Everything runs through [Task](https://taskfile.dev/), not `go` directly:

```sh
task check        # fmt + lint + test — before every commit; also what CI runs
task test          # go test ./... -race
task test:unit      # fast tests only (-short, without zip/gzip bomb tests)
task lint           # golangci-lint run
task build           # binary into bin/
task --list          # all tasks
```

`task check` **must be green** before a change counts as done.

## Architecture — dependency direction is binding

Clean Architecture, four layers, dependencies point only inward:

```
cmd/dmarc-analyzer  →  internal/web  →  internal/app  →  internal/domain
                        internal/infra ─────────────────↗
```

- `internal/domain/*`: pure business logic, **no** imports outside the
  standard library. No `encoding/xml` import here, no SQL.
- `internal/app/*`: use cases, depend only on domain ports (interfaces).
- `internal/infra/*`: adapters, implement the domain ports against
  concrete technology (SQLite, IMAP, dmarcxml, `envconfig` for ENV values).
- `internal/web/*`: Go `html/template` + Chart.js (`static/charts.js`),
  calls exclusively into use cases from `internal/app`. Chart logic
  (aggregation, filters, drill-down targets) belongs in Go — `charts.js`
  receives already-prepared JSON data and stays deliberately thin. Also
  contains the OIDC login flow (`auth.go`/`oidc.go`) — deliberately here
  and not in `internal/app`, because sessions/cookies are pure web-delivery
  concerns, not business logic.
- `cmd/dmarc-analyzer`: the only place where adapters are wired to use
  cases (composition root) — also reads the ENV configuration here
  (`envconfig.Load()`).

Details and rationale (SOLID/DDD/Clean Code) in `docs/architecture.md`.

## Web UI: no inline scripts, every state change is a POST with CSRF

- The Content Security Policy forbids `unsafe-inline` (see
  `internal/web/middleware.go`, `middleware_test.go`) — every behavior
  belongs in `internal/web/static/*.js`, never in a `<script>` tag or an
  `onclick` attribute in a template.
- Every state change (form, button with a side effect) is a POST with a
  CSRF token (`templateFuncs["csrfToken"]`, checked by `requireCSRF`) —
  never a GET with a side effect.
- Every new page/new template state (empty, error, populated) belongs
  rendered with `httptest` and parsed as HTML in a test — a template
  error should surface in the test, not first in the browser.

## Naming convention: no package stutter

Domain types are not named `report.ReportKey`, but `report.Key` — otherwise
the qualified name stutters (`report.ReportKey`). Exception: when the short
form would collide with a field name (`report.ReportID` stays that way
because `AggregateReport.ID ID` would be unreadable — likewise
`account.AccountID` because of `MailAccount.ID ID` — see the comment in the
respective code). `golangci-lint` (revive's `exported` rule) enforces this;
don't blanket-suppress it with `//nolint`, rename instead, unless a real
collision prevents it.

## SQLite: avoid IN clauses with many values

A `WHERE x IN (?, ?, ..., ?)` clause with one placeholder per row scales
badly — with 10,000 values, just preparing the statement alone took >3s
(measured in `internal/infra/sqlite`, see `reportrecords.go`). When the
filter value (here: `report_id`) is already known, join over the foreign-key
relationship instead
(`JOIN records ON records.id = child.record_id WHERE records.report_id = ?`)
rather than first loading all IDs and building an IN clause from them. When
adding new batch-loading functions in `internal/infra/sqlite`, always test
with realistically large data volumes (see `reportperf_test.go`), not just a
handful of test rows — the problem doesn't show up there.

## Dependencies

**Don't add anything to `go.mod` that isn't actually imported.** `go.mod`
grows organically when code genuinely imports a library — not ahead of
time. Concrete pinned versions live in `docs/DEPENDENCIES.md`, not in
`go.mod`. `task tidy` removes unused requires anyway.

## Tests

- `github.com/stretchr/testify/require` for assertions, no hand-rolled
  `if err != nil { t.Fatalf(...) }` chains.
- Table-driven with `t.Run(...)` where several cases check the same logic.
- `t.Parallel()`, unless the test uses `t.Setenv()` (incompatible).
- Mark slow tests (> ~1 s, e.g. zip/gzip bombs with >100 MB of test data)
  with `if testing.Short() { t.Skip(...) }` so `task test:unit` stays fast.
- Fixtures for the DMARC parser: `testdata/reports/<style>/...`, IPs from
  the RFC 5737 range (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`),
  domains as `example.com`/`example.org` — never real addresses or domains.
- `internal/web` tests against OIDC-protected routes need a real server:
  `New(ctx, deps, opts)` makes a real OIDC discovery request against
  `opts.OIDC.IssuerURL` at startup. For this there's
  `internal/web/fakeoidc_test.go` (`newFakeOIDCProvider`) — a minimal,
  local identity provider (discovery/JWKS/authorize/token, genuinely
  signed ID tokens), not a mock of the client logic. See
  `server_test.go` (`newTestServer`, `loginViaFakeOIDC`,
  `beginFakeOIDCLogin`) for the pattern.

## Other important things

- **Don't commit on your own.** Finish changes, show `git status`/
  `git diff`, but only run `git commit` on explicit instruction.
- RFC 7489 is the authoritative reference for everything around DMARC
  aggregate reports (field names, enum values, defaults like `pct=100` or
  `adkim/aspf=r` when the field is missing).
- Unknown/RFC-deviating enum values are never discarded, but mapped to an
  `Unknown` value — providers don't always follow the RFC, and import must
  not fail because of that.

## Masked types (Secret & co.): don't forget `%#v`

`String()`/`MarshalJSON()` aren't enough to make a value truly unreadable —
`%#v` uses `fmt.GoStringer` (`GoString()`), not `fmt.Stringer`. Without a
custom `GoString()` method, `%#v` on a struct with a `[]byte` field shows
the raw bytes hex-encoded (trivially reversible), even though `%v`/`%+v`
already mask correctly — found in `internal/domain/account.Secret` (see
`secret.go`/`secret_test.go`). For every new masked type: add a
`GoString()` too, and in the test don't just check for the plaintext
substring, but also for the hex-encoded form
(`encoding/hex.EncodeToString`) — otherwise this exact gap won't show up in
the test.

## go-imap/v2 is beta — known pitfalls

- `imapmemserver.User.Append(mailbox, reader, nil)` **panics** (nil pointer,
  `mailbox.go: appendBytes` reads `options.Time`/`options.Flags` without
  checking). Always pass `&imap.AppendOptions{}` instead of `nil`. See
  `internal/infra/imap/testserver_test.go`.
- The `Dial*` functions in `imapclient` aren't context-aware. Build your own
  connection via `net.Dialer`/`tls.Dialer` with `DialContext` and hand it to
  `imapclient.New(conn, opts)` (see `dial.go`); make blocking commands
  (`Login().Wait()`, `Fetch()`/`Next()`) context-aware via `runCtx()`
  (closes the connection on `ctx.Done()`, which makes the blocking call
  return with an error).
- On every `go get` within this module: don't forget `go mod tidy` —
  otherwise freshly, directly imported packages incorrectly stay marked as
  `// indirect` (happens when `go get` resolves several transitive
  dependencies in one go).

## Session cookies in tests: `net/http/cookiejar` drops Secure cookies on `http://` test servers

`auth.go` sets the session cookie with `Secure: true` (production runs
behind a TLS-terminating reverse proxy). `net/http/cookiejar` correctly
implements RFC 6265: a Secure cookie stored via `SetCookies` is **silently
not returned** on a later `Cookies(u)` call for an `http://` URL — a test
client built with `http.Client{Jar: cookiejar.New(nil)}` therefore loses
the session after the first redirect, with no error occurring (just a 303
to `/anmelden` instead of the expected page). Two solutions, depending on
the kind of test: `internal/web/api_charts_test.go` (`sessionTransport`)
bypasses cookie handling entirely and injects the cookie via a custom
`http.RoundTripper` — for handler tests that don't go through a real login
(`authenticatedClient`). For tests that must go through the real OIDC
redirect dance (`server_test.go`, `testCookieJar`), a deliberately
simplified `http.CookieJar` that ignores Secure/Domain/Path. Don't replace
either pattern with a plain `cookiejar.New(nil)`.

## Chart colors (`internal/web/static/app.css`) follow the `dataviz` skill's reference palette

The concrete hex values in `:root`/`prefers-color-scheme: dark` (primary
blue `#2a78d6`/`#3987e5`, status green/yellow/red
`#0ca30c`/`#fab219`/`#d03b3b`) come unchanged from the reference palette of
the `dataviz` skill (`references/palette.md`) — **not** invented freely.
`internal/web/static/charts.js` uses the same status colors (pass/fail/
disposition charts) via the same CSS variables (`cssVar("--status-good")`
etc.) — the UI and charts speak the same color language as a result. Before
changing these values: load the `dataviz` skill and run
`scripts/validate_palette.js` against the new values (don't decide by eye)
— see the instructions there. Per the skill, status colors are deliberately
**fixed regardless of mode** (not different per light/dark), while
primary color/surfaces do vary.

## Chart.js `responsive: true` + `maintainAspectRatio: false`: the canvas needs a parent with a fixed height

In responsive mode, Chart.js observes the **direct parent node** of the
`<canvas>` via `ResizeObserver` to adjust its height/width. If that parent
node has no explicit height independent of the canvas itself (e.g. because,
like an ordinary `.chart-card`, it only sets `padding`/`border` and its
height is otherwise "auto", i.e. derived from its content), a feedback loop
results: the canvas grows → the parent node grows with it, because its
height depends on the canvas content → the ResizeObserver sees the new
(larger) parent height and grows the canvas again → unbounded downward
growth, visible in practice as "a chart expands endlessly while the page
loads" (found on the dashboard with the `chart-verlauf` chart, which was a
direct child of `.chart-card`).

Setting `canvas.style.height = "…px"` directly in JavaScript **does not fix
this** — on the contrary, it collides with Chart.js's own management of
canvas width/height and can even trigger the loop. The height instead
belongs on a dedicated wrapper `<div>` as the canvas's direct parent node
(`position: relative` plus a fixed height, or one set dynamically via JS),
never on the canvas itself. See `internal/web/static/app.css`
(`.chart-canvas-wrap`, with a detailed comment) and
`internal/web/templates/pages/dashboard.html` — each of the four dashboard
charts has its own `<div class="chart-canvas-wrap" id="chart-<name>-wrap">`
around the `<canvas>`. For a fixed height (here: message volume,
disposition), CSS is enough; for a data-dependent height (here: top
sending sources, heatmap — more rows/columns need more space)
`internal/web/static/charts.js` sets the height at runtime via
`document.getElementById("chart-<name>-wrap").style.height = …` on the
wrapper, not on the canvas. Adopt this pattern for every new Chart.js chart
with `maintainAspectRatio: false`, otherwise the bug recurs.
