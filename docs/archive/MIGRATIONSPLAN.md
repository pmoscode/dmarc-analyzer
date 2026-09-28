# Migration plan — Fyne UI → embedded web UI

> As of: 2026-09-17. Complements `UMSETZUNGSPLAN.md` and belongs **before
> WP 7** there (packaging and polish depend on the outcome).
> Progress: M0–M5 implemented (see section 10), M6 open.

## 1. Reason and goal

The Fyne UI still feels heavy ("clunky") despite rework. The goal is a
**web UI that lives entirely inside the binary**: on startup, a local
HTTP server runs, and the default browser automatically opens the main
page. No web server to install, no external files, no internet access
for the UI.

**Requirement this changes:** `FEATURES.md` explicitly names Fyne as the
UI framework and "Fyne framework best practices" under "Non features";
`IMPLEMENTIERUNG.md` section 1.2 treated that as binding. This migration
deliberately replaces that requirement — `FEATURES.md` is updated
accordingly in M5 (see section 12).

### What the migration additionally brings

| Benefit | Why |
| --- | --- |
| **No more CGO** | Fyne is the only CGO dependency. Checked on 2026-09-17: all packages except `internal/ui` build with `CGO_ENABLED=0` for `linux/amd64`, `windows/amd64`, and `darwin/arm64`. Releases then become cross-compiles on one machine. |
| **Interactive charts instead of static images** | Today charts are server-rendered PNGs: no tooltip, no show/hide, no zoom, no click. Going forward, Chart.js draws in the browser — with tooltips, a toggleable legend, zoom in the time series, and **drill-down**: clicking a day, a sending source, a heatmap cell, or a donut segment opens the matching reports. |
| **Charts adapt to light/dark** | Today the PNGs have a fixed white background (`flattenOnWhite`) — bright boxes in dark mode. Chart.js reads colors from CSS variables and redraws on a color-scheme change. |
| **Real tooltips** | Fyne v2.8 has none; today a "?" button substitutes. In the browser: hover tooltips on charts and terms. |
| **Fewer workarounds** | `internal/infra/charts` contains several go-chart workarounds (text wrapping, single-value donut, transparent borders), `internal/ui` several Fyne traps (see `AGENTS.md`). Both go away. |
| **Bookmarks and back button** | Filters live in the URL (`?zeitraum=30&domain=…`). |

## 2. What stays, what changes

Clean Architecture carries the migration: **only the presentation layer
is swapped.** Domain, use cases, and adapters stay, with four small,
targeted extensions (section 9).

```
                 today                                   after the migration
cmd/dmarc-analyzer ──▶ internal/ui (Fyne)      cmd/dmarc-analyzer ──▶ internal/web (net/http)
                          │                                                 │
                          ▼                                                 ▼
                   internal/app/*        (unchanged, + sync progress, import from bytes)
                          │
                          ▼
            internal/domain/*  ◀──  internal/infra/*   (unchanged, + lockable key store)
```

| Area | Treatment |
| --- | --- |
| `internal/domain/*`, `internal/app/*`, `internal/infra/*` | stay; extensions see section 9 |
| CLI subcommands (`sync`, `import`, `stats`, `account`) | stay unchanged |
| `internal/ui/i18n` (plain constants) | moves to `internal/web/i18n`, content stays |
| `internal/ui/glossary/terms.go` (plain data) | moves to `internal/web/glossary` |
| Color values from `internal/ui/theme.go` | become CSS variables (light/dark), values stay |
| the rest of `internal/ui/*` (~4,900 lines including tests) | deleted in M5 |
| `internal/infra/charts` (go-chart), port `analysis.ChartRenderer`, `exportdata.WriteChartPNG` | go away — charts are produced in the browser (E-2) |

## 3. Target picture in operation

| Situation | Behavior |
| --- | --- |
| `dmarc-analyzer` with no arguments (also double-click) | start a server on `127.0.0.1` on a random free port, open the browser with a one-time login link, also print the address to the console |
| Program already running | a second launch detects the running instance, has it generate a fresh one-time link, opens it in the browser, and exits immediately |
| No accounts present | `/` redirects to first-run setup |
| `dmarc-analyzer serve --adresse 127.0.0.1:8080 --kein-browser` | fixed port, no browser launch (development, SSH session) |
| Quit | Ctrl+C / SIGTERM (see E-3) — no auto-exit, no button in the UI |
| No OS keychain (Linux without a secret service) | the UI first shows an unlock page for the master passphrase — today the program asks on the console, which doesn't exist on a double-click launch |
| CLI subcommands | unchanged, no server |

**Opening the browser** with no extra dependency via `os/exec`: `open`
(macOS), `xdg-open` (Linux), `rundll32 url.dll,FileProtocolHandler`
(Windows). If that fails, the address stays on the console — no abort.

## 4. Decisions

The recommendation is preselected in each row. Please confirm or change
before M0 begins.

| No. | Question | Recommendation | Alternative | Rationale |
| --- | --- | --- | --- | --- |
| E-1 | Frontend technology | **`html/template` (standard library) + htmx** for pages, tables, and forms; custom JavaScript only for the charts (section 6a); everything embedded, **no bundler** | SPA (Svelte/Vue/React) with a Vite build | No Node tooling in development and CI, `go build` stays the only build step. Server-side rendering covers tables and forms well; the interactivity that actually matters comes from the charts. An SPA doubles up ecosystems and tests. |
| E-2 | Charts | **Fixed: Chart.js in the browser** (requirement), augmented with the plugin `chartjs-chart-matrix` for the heatmap and `chartjs-plugin-zoom` for the time series. The server only delivers data as JSON. go-chart, `internal/infra/charts`, and `analysis.ChartRenderer` go away. | Apache ECharts (heatmap, zoom, and image export built in, but ≈1 MB and its own theming system) | Chart.js is small, widespread, and brings tooltips, toggleable legends, and click events. The heatmap isn't a core Chart.js type — hence the matrix plugin. **Fallback:** if the spike (M0) shows the plugin combination doesn't hold up, switch to ECharts; only `charts.js` and the JSON shape are affected. |
| E-3 | Lifecycle | **Only Ctrl+C / SIGTERM** (`context.Context` with `signal.NotifyContext`, clean shutdown via `http.Server.Shutdown`) — no auto-exit, no "quit" button, no tray icon | "quit" button + auto-exit after idle · tray icon | The process should behave like an ordinary server service, not like a desktop app with its own window — planned use also as a Docker image (see below), where the lifecycle comes from the container orchestrator (`docker stop` sends SIGTERM) and "count open browser tabs" isn't a sensible basis. Also simplifies the server considerably along the way: no SSE connection counting, no timer, no `--dauerhaft` flag needed — this is just the only mode from now on. WP 7 (background sync) benefits directly: the process runs anyway until it's stopped. |
| E-4 | Port | **random** (`127.0.0.1:0`), fixed only via a flag | a fixed default port | No conflicts; bookmarks still work via single-instance detection, because every launch opens the correct address. |
| E-5 | Transition | **Build the web UI in parallel in `internal/web`**, Fyne stays the default until feature parity (M3), then switch over and delete Fyne in M5 | switch over immediately | The program stays usable at all times. There are no users before 1.0 who'd need a parallel offering — Fyne therefore doesn't stay around as a permanent option. |
| E-6 | New possibilities in this migration | **Include file import via upload/drag & drop** (suggestion 11.1 was previously CLI-only); filters in the URL; sortable table headers | 1:1 transfer only | The import dialog replaces the Fyne file dialog anyway; the extra effort is small. |
| E-7 | Sourcing htmx, Chart.js, and plugins | **Keep minified UMD files in the repository** (`internal/web/static/vendor/`), pin versions and licenses (all MIT) in `docs/DEPENDENCIES.md`; research the exact versions and plugin compatibility with the Chart.js major version when integrating | CDN | Works offline, fits the Content Security Policy `default-src 'self'`. |
| E-8 | Browser tests | **A lean smoke test with `chromedp`** (Go, no Node): load the overview, check that all four charts are drawn and the console reports no errors, trigger one drill-down click. Runs locally and on the CI's Linux runner; handler tests with `httptest` cover logic and security. | no browser tests · Playwright (needs Node) | With Chart.js, real logic lives in JavaScript (data mapping, colors, click targets) that `httptest` can't see. Whether Chrome is present on the CI runner or needs installing is checked in M2. |

## 5. Security of the local server

A server on `127.0.0.1` is **not automatically private**: other users of
the same machine can connect, and any open web page can send requests to
`127.0.0.1` (CSRF, DNS rebinding). The UI manages credentials — hence:

| Measure | Implementation |
| --- | --- |
| Loopback only | Bind exclusively to `127.0.0.1`; other addresses are rejected. **Tension with the planned Docker deployment (E-3):** a process that only binds to `127.0.0.1` inside a container is **not** reachable from outside via `docker run -p …` — Docker forwards to the container's network interface, not to its loopback. For the Docker case, a different binding (`0.0.0.0` in the container, or `--network=host`) and thus a different threat model will be needed once the Docker image comes up: the server would then potentially be reachable from other machines on the network, not just from processes on the same machine. This migration only builds the local (loopback) model; **a later Docker packaging needs its own security addendum** (among other things: is the one-time-link approach still enough, or does it then need a real login with password and TLS?) — deliberately not part of this plan, see open question 4. |
| Instance secret | 32 random bytes (`crypto/rand`) at startup, stored in `instance.json` in the config directory with permissions `0600`, together with port and PID |
| One-time login link | The browser doesn't get the secret, but a **one-time-valid code** (valid for 60s). `/anmelden?code=…` exchanges it for a session cookie (`HttpOnly`, `SameSite=Strict`) and redirects — history keeps only a used-up code. A second program launch fetches a fresh code using the instance secret. |
| Every request authenticated | Without a valid session cookie: 401 and a notice page "please open via the program" |
| DNS rebinding | The `Host` header must be exactly `127.0.0.1:<port>` or `localhost:<port>` |
| CSRF | State-changing requests only via POST, with a CSRF token (form field or `hx-headers`) **and** checking `Origin`/`Sec-Fetch-Site` |
| Content Security Policy | `default-src 'self'; frame-ancestors 'none'; form-action 'self'` — no inline JavaScript, no external sources. Whether Chart.js and the plugins can do without `'unsafe-inline'` in `style-src` is checked in the spike (M0), not assumed. |
| JSON endpoints | Same session and `Host` checks as pages; GET only, no state change; `Content-Type: application/json` and `X-Content-Type-Options: nosniff` |
| Credentials | Passwords only via POST, never sent back to the browser, never logged; overwrite `account.Secret` with `Zero()` right after use, as before |
| Uploads | Size limit via `http.MaxBytesReader` (suggestion: 50 MB per file); the 100 MB unpack limit from WP 1 also applies |
| Cleanup | Delete `instance.json` on exit; an orphaned entry (dead process) is taken over on the next launch |

## 6. Package structure

```
internal/web/
├── server.go          # http.Server, start/stop via signal.NotifyContext, single instance
├── browser.go         # open the browser per OS
├── auth.go            # instance secret, one-time codes, session, CSRF
├── middleware.go      # host check, security headers, recover, logging
├── routes.go          # routing (net/http ServeMux with method patterns)
├── handlers_*.go       # one per view: dashboard, reports, sources, accounts, onboarding, sync, import, export
├── api_charts.go       # JSON data for the charts (section 6a)
├── views.go            # template loading (embedded; from disk with --entwicklung)
├── i18n/                # moved from internal/ui/i18n
├── glossary/            # moved from internal/ui/glossary (data only)
├── templates/           # layout.html, partials/*.html, pages/*.html
└── static/
    ├── app.css          # theme variables light/dark
    ├── app.js           # tooltips for terms, SSE (sync progress), dialogs
    ├── charts.js         # Chart.js config, colors from CSS variables, drill-down
    └── vendor/           # htmx, chart.js, chartjs-chart-matrix, chartjs-plugin-zoom (each .min.js + LICENSE)
```

`templates/` and `static/` are embedded via `//go:embed`. With
`--entwicklung`, the server instead reads them from disk — changes to
HTML/CSS/JS become visible without a rebuild.

## 6a. Charts with Chart.js

**Division of labor:** the server delivers already-prepared data; the
browser draws. Aggregation, filtering, and enrichment (PTR, service
name) stay in Go and thus testable — `charts.js` only maps data and
reacts to clicks.

| Chart | Chart.js type | Interaction | Drill-down on click |
| --- | --- | --- | --- |
| Message volume per day, stacked pass/fail | `bar` (stacked) | Tooltip with count and share per day; legend toggles pass/fail on and off; zoom and pan for long periods | Reports of that day |
| Top sending sources | `bar` (horizontal, so long service/hostnames stay readable) | Tooltip with IP, hostname, service, volume, pass rate | Sending-sources view, or reports filtered to that IP |
| Distribution by disposition | `doughnut` | Tooltip with count and share; segments hideable via the legend | Reports with that disposition (`report.Query.Disposition` already exists) |
| Sending source × day | `matrix` (plugin) | Tooltip with source, day, pass rate **and message count**; empty days clearly marked as "no data" | Reports from that source on that day |

**JSON endpoints** (same filter parameters as the page, e.g.
`?zeitraum=30&domain=example.com`):

| Path | Content |
| --- | --- |
| `/api/diagramme/verlauf` | Days with `pass`, `fail` |
| `/api/diagramme/quellen` | Sources with `ip`, `label`, `total`, `passRate` |
| `/api/diagramme/disposition` | Shares per disposition |
| `/api/diagramme/heatmap` | Sources, days, cells with `passRate`, `total`, `hasData` |

Every response already includes, for every data point, the **drill-down
target URL** (built by the server) — `charts.js` doesn't need to know any
filter logic.

**Design following the `dataviz` skill:**

- Colors come from the same CSS variables as the UI (validated reference
  palette: status colors for pass/fail/disposition, primary blue).
  `charts.js` reads them via `getComputedStyle` and redraws on a
  `prefers-color-scheme` change.
- Pass-rate color scale for top sources and heatmap as a gradient
  between the "critical" and "good" status colors; "no data" neutral
  gray.
- Slim bars with rounded ends, subtle gridlines, legend from two series
  on, labels in text color rather than series color.
- **Accessibility:** a `<canvas>` is empty to screen readers. Every chart
  gets a short description (`aria-label`) and a toggleable **table view**
  with the same data; drill-down targets there are regular links and
  thus reachable by keyboard.
- Empty periods show the existing empty state instead of an empty chart.

**Export:** PNG via `chart.toBase64Image()` in the browser, CSV of the
chart data from the table view. Server-side image export goes away.

**Domain addition:** `analysis.HeatmapCell` additionally gets the
message count (`Total`), so the tooltip can show it — the SQL
aggregation already computes it, it was just discarded so far.

## 7. Routes

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/anmelden` | Exchange a one-time code for a session |
| POST | `/intern/code` | Issue a new one-time code (only with the instance secret, for the second program launch) |
| GET | `/` | Overview (or redirect to first-run setup) |
| GET | `/api/diagramme/{verlauf,quellen,disposition,heatmap}` | Chart data as JSON (section 6a) |
| GET | `/berichte` | Report table; filters (also source IP, disposition, single day — for drill-down), sort, group as query parameters |
| GET | `/berichte/seite` | next page (htmx, keyset cursor) |
| GET | `/berichte/{id}` | Detail view (as a dialog via htmx and as its own page) |
| GET | `/quellen`, `/quellen/seite` | Sending sources |
| GET | `/glossar` | Glossary |
| GET | `/einstellungen` | Account list |
| POST | `/konten` · `/konten/test` · `/konten/ordner` · `/konten/{id}/test` · `/konten/{id}/loeschen` | Create, test connection, list folders, test a saved account, delete |
| GET/POST | `/einrichtung` | First-run setup in three steps |
| POST | `/abgleich` · `/abgleich/abbrechen` | Start/cancel sync |
| GET | `/ereignisse` | Server-Sent Events: sync progress, result |
| POST | `/import` | File upload |
| GET | `/export/berichte.csv` · `/export/quellen.csv` | Downloads (chart export runs in the browser) |
| GET/POST | `/entsperren` | Master passphrase for the file key store |
| POST | `/beenden` | Quit the program |
| GET | `/static/…` | Embedded files with cache headers |

## 8. Feature parity — transferring the existing UI

| Today (Fyne) | Going forward (web) | Note |
| --- | --- | --- |
| Main window, side navigation with icons, header | `layout.html` with a nav bar, header | icons as embedded SVG |
| Light/dark theme (`appTheme`) | CSS variables, `prefers-color-scheme` | same color values (dataviz reference palette) |
| Filter bar (time range, domain) | GET form, values in the URL | continues to act on the overview, reports, sources |
| Overview: 5 metric tiles, trend | Tiles as cards, trend arrow as an SVG icon | values formatted as today |
| 4 static charts (PNG), grouping 2 + 1 + 1 | 4 Chart.js charts, same grouping | new: tooltips, toggleable legend, zoom in the time series, click drill-down, table view per chart |
| Export a chart as PNG | PNG export in the browser, CSV from the table view | |
| Report table, "load more", grouping | HTML table, "load more" via htmx, sortable columns | keyset pagination stays |
| Report detail dialog | `<dialog>` element with a close button **and** Esc | the "can't be closed" bug can't recur |
| CSV export (loaded pages only) | CSV export of the **entire** filtered dataset, streamed page by page | improvement: no memory problem, since streamed |
| Sending sources with PTR/service | HTML table | |
| Settings: create/test/delete accounts | Form + list | validation server-side (logic taken over from `AccountForm.Validate`) |
| Folder picker (`SelectEntry`) | `<input list>` with `<datalist>`, a "list folders" button | |
| First-run setup (3 steps) | `/einrichtung` | |
| Sync button with progress and cancel | Button + progress display via SSE | sync keeps running server-side, even across page changes |
| Empty states, error banners with technical detail | Partials, detail in `<details>` | |
| Glossary dialog, "?" buttons | Glossary page + real tooltips (`popover` attribute, keyboard-reachable) | |
| — | **new:** file import via upload/drag & drop (E-6) | |

## 9. Extensions needed outside the presentation layer

1. **Sync progress** — `syncreports.UseCase.SyncAccount` today only
   returns a final result. Addition: an optional progress callback
   (processed/new/skipped/failed).
2. **Sync as a server-side job** — new package `internal/app/syncjob`:
   at most one run at a time, cancellable via context, progress for
   several listeners. Lives in the application layer because the
   background sync planned in WP 7 needs the same lock.
3. **Import from bytes** — `importfiles.UseCase` currently works with
   file paths. Addition `ImportData(ctx, filename, data)`; the internal
   splitting (`extractAttachments`) already works with bytes.
4. **Lockable key store** — `newCredentialStore()` currently prompts for
   the master passphrase on the console at startup. Addition: an
   `account.CredentialStore` that returns
   `account.ErrCredentialStoreLocked` until unlocked; the UI then
   redirects to `/entsperren`.
5. **Dismantle the chart port** — `analysis.ChartRenderer`,
   `internal/infra/charts`, `exportdata.WriteChartPNG`, and go-chart go
   away. `analysis.DailyVolume/SourceVolume/Heatmap` stay as data for the
   JSON endpoints; `HeatmapCell` gets `Total` (section 6a). An ADR in
   `docs/` records why the "adapter swap" planned in `IMPLEMENTIERUNG.md`
   8.2 becomes a dismantling here: on the web, charts are pure browser
   presentation, the server only delivers data.
6. **Drill-down filters** — `report.Query` already knows time range,
   domain, source IP, and disposition. To check in M2: whether "reports
   from one source on one day" is correctly captured with just time
   range + source IP (mapping via `date_begin`, see WP 6).

## 10. Work packages

Relative sizes: S (small), M (medium), L (large).

### M0 — Decisions and spike (S)

- [x] Decisions E-1 through E-8 confirmed — implicitly confirmed by
  implementing the respective recommendation (Chart.js, Ctrl+C/SIGTERM,
  random port, file import follows in M4, vendored JS bundles,
  `chromedp` smoke test follows in M2); not individually renegotiated.
- [x] `internal/web` with server, one-time login, host check, browser launch
- [x] One page: overview with metric tiles from `statistics.UseCase`
- [x] **Chart.js spike:** heatmap (matrix plugin) and time series with
  zoom from real data, tooltip, one drill-down click — under the
  planned Content Security Policy without `'unsafe-inline'`
- [x] Startup path behind a flag (`dmarc-analyzer web`), Fyne stays the default

**Done when:** a launch opens the browser, shows real metrics and two
interactive charts, and a request without a session or with a foreign
`Host` is rejected. If the Chart.js plugin combination doesn't hold up,
the decision falls here for the ECharts fallback (E-2). — **Reached.**

### M1 — Scaffolding (M)

- [x] Layout, navigation, header, light/dark theme as CSS variables —
  navigation currently has four placeholder pages (reports, sending
  sources, glossary, settings; content follows in M2/M3), theme
  unchanged from M0.
- [x] Filter bar with values in the URL — acts on the overview (metrics,
  both charts including drill-down targets); reports/sources adopt the
  same filter once they get real content in M2.
- [x] Security middleware complete (section 5), CSRF, CSP —
  `requireCSRF` (Origin/Sec-Fetch-Site + token) is already wrapped
  around the entire protected route tree, even though no form needs it
  yet (first consumer: the account form in M3).
- [x] Single instance (`instance.json`), a second launch opens the existing instance
- [x] Clean shutdown via Ctrl+C/SIGTERM (E-3, `signal.NotifyContext` +
  `http.Server.Shutdown`), `--kein-browser`, `--adresse`, `--entwicklung`
- [ ] `i18n` and glossary data moved; empty-state and error partials —
  **deferred to M5.** Actually moving them now would pull the packages
  out from under `internal/ui` (the default UI until M3, see E-5); an
  `internal/ui` → `internal/web` import would also be a forbidden
  dependency direction (AGENTS.md). Until then, the few texts already
  in use (navigation, filter bar) stay directly in `internal/web` as
  German literals — real double maintenance only starts once both UIs
  would actually need to run from the same i18n source, and that isn't
  the plan (E-5: switch over, not run in parallel). Empty-state/error
  partials also deferred: the only "empty" situation so far is the M1
  placeholder pages themselves.
- [x] htmx, Chart.js, and plugins integrated, versions and licenses
  pinned in `docs/DEPENDENCIES.md`

**Done when:** navigation between empty pages works in light and dark, a
second launch just opens another tab, and Ctrl+C/SIGTERM shuts the
process down cleanly (open requests finish, `instance.json` removed). —
**Reached** (visual verification in real browsers per M6 is still
outstanding, see the caveat there about no display in this environment).

### M2 — Read-only views (L)

- [x] JSON endpoints for all four charts including drill-down URLs; `HeatmapCell.Total`
- [x] `charts.js`: four charts per section 6a, colors from CSS variables,
  redraw on light/dark switch; palette **not** checked against the
  `dataviz` skill's `scripts/validate_palette.js` — the skill wasn't
  available in this session (only `customize-opencode` was loaded).
  Only the CSS variables already validated in M0/M1
  (`--status-good/-warning/-critical`, `--primary`) were reused; no new
  color values invented. Before a release: load the skill and catch up
  on validation.
- [x] Overview complete: tiles, trend, four charts in the existing
  grouping (time series+donut side by side, top sending sources and
  heatmap each a full row), tooltips, legend, zoom, drill-down, table
  view per chart
- [x] Report table with sorting (organization/domain/period, clickable
  column headers), grouping (none/domain/organization), "load more"
  (htmx, keyset cursor), detail view (metadata/policy/sending sources),
  and the drill-down filters (source IP, disposition, day as from/to);
  extension 9.6 checked — overlap semantics (`date_begin < to AND
  date_end > from`) confirmed against the real adapter, no adjustment
  needed. **Not implemented:** the detail view is its own page, not a
  `<dialog>` overlay via htmx (section 8 envisioned a dialog element) —
  a deliberate scope decision this session, to finish M2 in a
  reasonable time; functionally complete (back link, own URL,
  keyboard-/screen-reader-friendly), just without the modal polish. Can
  be retrofitted in M6.
- [x] Sending-sources table (source IP, messages, pass rate, PTR
  hostname, detected service; sortable by source IP/messages; "load more")
- [x] Glossary page and term tooltips — data lives as
  `internal/web/glossary` (a copy of `internal/ui/glossary/terms.go`,
  see the M1 rationale on the i18n/glossary move); "?" links on the
  overview next to tiles/chart titles point to `/glossar#<slug>`.
- [ ] `chromedp` smoke test (E-8) — **deferred.** No Chrome/Chromium is
  installed in this sandbox (`which google-chrome chromium
  chromium-browser` returns nothing, no Chrome install under
  `/Applications`); a smoke test couldn't be written AND verified here.
  Everything the smoke test would additionally have checked beyond the
  existing `httptest` handler tests (Chart.js actually draws, no
  console errors, a real click navigates) has instead been verified
  manually with the built binary + `curl` against imported test
  reports (login, overview, reports including sort/filter/pagination,
  sending sources, detail view) — that doesn't replace a real browser
  test, though. Catch up once an environment with Chrome is available
  (a local dev machine or a CI runner with `chromium`/`google-chrome`
  preinstalled).

**Done when:** all read-only functions of the Fyne UI exist in the
browser (table in section 8), every chart click leads to the matching
reports, and with 100,000 records the report table and charts stay
smooth (a year-long heatmap: 10 sources × 365 days). — **Functionally
reached**, manually verified with imported test reports (no Fyne feature
from section 8 is missing anymore except the dialog overlay, see above).
**Not verified:** performance at 100,000 records — report/sending-source
queries go through the same keyset repositories already tested with
realistic data volumes in WP 2/4 (`internal/infra/sqlite`, see
`reportperf_test.go`); the web handlers themselves never load more than
one page (`Limit: 50`) — a dedicated HTTP-level load test wasn't due in
this session. A real browser smoke test is missing (see the chromedp
point above).

### M3 — Write flows (L)

- [x] Extensions 9.1, 9.2, 9.4 implemented and tested:
  - 9.1: `syncreports.UseCase.SyncAccount` gets an additional
    `onProgress` parameter (`OnProgress func(Progress)`, nil allowed) —
    a parameter rather than a struct field, so the shared
    `*syncreports.UseCase` can't be confused by parallel calls with
    different progress callbacks. Existing callers (CLI, Fyne,
    onboarding) pass `nil`.
  - 9.2: new package `internal/app/syncjob` (`Runner`) — at most one run
    at a time (`Start`/`ErrAlreadyRunning`), cancellation via
    `context.CancelFunc` (`Cancel`), progress for any number of
    listeners (`Subscribe`, a buffered channel with "keep the latest
    state" semantics). Attaches to `syncreports.UseCase` via a dedicated
    `Syncer` interface rather than the concrete struct — simplifies
    tests considerably (a fake instead of a fully wired IMAP stack). A
    real bug not foreseen when writing this plan was found here and
    fixed with a regression test: Cancel() during the LAST (or only)
    account was incorrectly reported as `done` instead of `cancelled`
    (the cancellation check only sat before the NEXT account each
    time).
  - 9.4: `account.ErrCredentialStoreLocked` (new sentinel) +
    `internal/infra/keyring.LockableFileStore` — starts locked, returns
    this error until `Unlock(passphrase)`; `newCredentialStore` in
    `cmd/dmarc-analyzer/wire.go` uses it only for `cmd == "web"` without
    an OS keychain (CLI commands still prompt blockingly on the
    console, which remains correct there). Actually observed in this
    sandbox (no keychain available) and the complete
    unlock→setup path was thus run for real, not just tested with fakes.
- [x] Settings (`/einstellungen`): create accounts (`POST /konten`),
  test (`POST /konten/{id}/test`), delete
  (`POST /konten/{id}/loeschen`), list folders
  (`POST /konten/ordner`, fills a `<datalist>` for the mailbox field).
  No connection test before saving (parity with
  `internal/ui/settings.View` — only first-run setup tests beforehand,
  see below). The password is never echoed in the re-shown form
  (validation errors, folder list) — standard convention against
  plaintext passwords in the HTML source, even though that requires
  retyping after "list folders".
- [x] First-run setup in three steps (`/einrichtung`, server-side state
  between steps via `onboardingState` — deliberately not via hidden
  form fields, for the same password reason):
  1. enter account, 2. test connection (save only on success, parity
  with `internal/ui/onboarding.Wizard`), 3. trigger the first sync or
  skip it. `/` redirects to `/einrichtung` as long as no accounts exist.
- [x] Sync with progress (SSE, `GET /ereignisse`) and cancel
  (`POST /abgleich/abbrechen`) — the sync button plus progress display
  sits in `layout.html` (every page), not just on the overview: the
  sync affects all accounts at once and should stay visible across page
  changes (`static/app.js`, no framework, plain `EventSource`).
  **Simplified compared to the Fyne UI:** no separate "show first sync
  result" step after first-run setup (`showDoneStep` equivalent) — the
  SSE display in the header handles that continuously, even across the
  redirect to the overview.
- [x] Unlock page for the file key store (`/entsperren`) — additionally
  verifies an entered passphrase against an actually stored secret, if
  an account already exists (`Unlock()` itself doesn't actively check
  this, see the store documentation); on failure it locks again instead
  of silently accepting a wrong passphrase — a real bug of this kind
  (any passphrase was accepted if no secret existed yet) was found and
  fixed while testing (`account.ErrCredentialNotFound` isn't proof of a
  wrong passphrase, any other error is).
- [x] **Switchover:** `dmarc-analyzer` with no arguments starts the web
  UI (`main.go` passes `["web"]` through to `run()`). The old Fyne UI
  stays reachable until the removal in M5 via the no-longer-advertised
  `gui` subcommand (a manual comparison/fallback path, see
  `cmd_gui.go`).

**Done when:** a user sets up an account in the browser without
documentation, picks a subfolder, syncs, and sees their reports — the
same acceptance criterion as WP 5. **Reached** (manually walked through
with the built binary: unlock → first-run setup → account → connection
test → trigger sync → settings → test/delete account, each with the
expected results including real, meaningful error messages for
unreachable test hostnames). The folder picker itself wasn't verified
against a real IMAP server (none available in this sandbox) — the
underlying `manageaccount.UseCase.ListMailboxes` was already tested
against a real server in WP 3/4; new here is only the form wiring
(`POST /konten/ordner`), which is covered with a fake `MessageSource`.

### M4 — Import and export (M)

- [x] Extension 9.3: `importfiles.UseCase.ImportData(ctx, filename,
  data)` — same logic as `ImportFile`, just without the detour through
  disk (`ImportFile` now calls `ImportData` itself). Web route
  `POST /import` (multipart upload, multiple files at once), page
  `GET /import` with a drag-and-drop area (native `<input
  type="file">`, JS only for visual feedback while dragging and for
  filename display, see `static/app.js`) and a result display
  (new/skipped/failed) after uploading. Size limit as noted in the plan:
  50 MB per file, plus an overall cap on the whole request
  (`http.MaxBytesReader`).
  **A bug found and fixed along the way (not part of the original
  extension 9.3, but directly required by the "done when" criterion
  below):** `.zip` attachments containing multiple reports were only
  partially imported by both `importfiles.UseCase` and
  `syncreports.UseCase` — both called only `ReportParser.Parse()`,
  which by its own documentation returns only the *first* report
  (`internal/infra/dmarcxml.Parser.ParseAll` would have been needed for
  multiple). A comment in `importfiles/usecase.go` even hinted at this
  ("possibly several reports for .zip") without the code actually
  delivering on it. Fix: a new optional interface
  `domainsync.MultiReportParser` (same pattern as the already-existing
  `MailboxLister`) plus `domainsync.ParseAttachment()`, which uses it
  via a type assertion, falling back to `Parse()` otherwise — now used
  by both use cases. Found via a new test with a real multi-report zip
  file (`testdata/reports/multi/two_reports.zip`), which failed without
  the fix; regression tests in all three affected packages
  (`importfiles`, `syncreports`, `internal/web`).
- [x] CSV export of the entire filtered dataset, streamed —
  `GET /export/berichte.csv` and `GET /export/quellen.csv`, each with
  the same filter as the currently displayed table. Internally lazy-
  loads page by page (`exportPageSize` 500, keyset cursor like the
  table view itself) and writes/flushes each page directly into the
  HTTP response — the entire filtered dataset is never held fully in
  memory. `exportdata.WriteReportsCSV`/`WriteSourceStatsCSV` were split
  into header/row building blocks
  (`WriteReportsCSVHeader`/`WriteReportCSVRow` etc.) for this; the
  existing functions remain as convenience wrappers for the (still
  unstreamed) case "export a page already loaded".
- [x] Chart export as PNG (browser) and CSV (table view) — two buttons
  per chart ("export PNG"/"export CSV") in `dashboard.html`; `charts.js`
  keeps a small registry for this (`chartExports`) of chart key →
  Chart.js object + CSV row function, PNG via
  `chart.toBase64Image()`, CSV purely client-side from the same data
  the table view also shows. No server-side image export (as the plan
  envisioned).

**Done when:** a `.zip` with several reports can be imported via drag &
drop, and exporting 100,000 records runs without noticeable memory
growth. **Reached:** zip multi-import verified with the real CLI binary
against `testdata/reports/multi/two_reports.zip` (2 new, previously —
before the bugfix described above — only 1). CSV export streaming
verified with a fake repository that delivers requested pages one by
one only on demand (`TestHandleExportReportsCSV_StreamsAllPages`) — a
real 100,000-row load test didn't run in this session (same limitation
as with M2: the underlying keyset repositories were already tested with
realistic data volumes in WP 2/4, the streaming layer itself provably
never loads more than one page at a time).
**Not verified with the real binary in a browser:** web upload
(`POST /import`) and chart export buttons — in this sandbox,
`dmarc-analyzer web` hangs indefinitely at startup in
`keyring.IsAvailable()` (OS keychain access from a freshly compiled,
ad-hoc-signed binary with no graphical session that could answer a
permission dialog — confirmed: `go test` against the same function
responds in <100ms, the compiled `bin/dmarc-analyzer` doesn't within
several minutes). A pure sandbox/session artifact, no change to
production code — the complete HTTP layer (multipart upload including
real `dmarcxml`/`mailmime`, size limits, CSRF, CSV streaming) is instead
covered via `httptest` (see `internal/web/handlers_import_test.go`,
`handlers_export_test.go`); the Chart.js export buttons are pure browser
JavaScript with no server counterpart and thus only checkable via a real
browser anyway (the same `chromedp` caveat already documented in M2).

### M5 — Switchover and removal (M)

- [x] `internal/ui` and `cmd/dmarc-analyzer/cmd_gui.go` deleted; Fyne out
  of `go.mod` — 40 files/4,916 lines of `internal/ui` removed, `main.go`'s
  `subcommands` map cleaned of the (already nowhere-documented) entry
  `"gui"`. `go mod tidy` then automatically removed `fyne.io/*` and all
  transitive dependencies only needed for it (among others `go-gl/*`,
  `go-text/*`, `srwiley/*`, `nfnt/resize`, `rymdport/portal`,
  `fyne-io/*`) from `go.mod`.
- [x] Extension 9.5 (chart port, `internal/infra/charts`, go-chart
  removed) — the `analysis.ChartRenderer` interface removed from
  `internal/domain/analysis/charts.go` (the plain data types
  `DailyVolume`/`SourceVolume`/`Heatmap`/`HeatmapCell` stay, `Total` was
  already present); `internal/infra/charts` (4 files, the go-chart
  implementation including manual heatmap drawing with `image/draw`) as
  well as `internal/app/exportdata/png.go`+`png_test.go`
  (`WriteChartPNG`, whose only caller was the now-deleted Fyne
  dashboard) deleted. `github.com/wcharczuk/go-chart/v2` thus also
  removed via `go mod tidy`. Rationale:
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.
- [x] Build with `CGO_ENABLED=0`; `task release` as a cross-compile for
  macOS (arm64, amd64), Windows (amd64), Linux (amd64, arm64) with
  checksums — `Taskfile.yml` now has `release:darwin`/
  `release:windows`/`release:linux`/`release`, all as plain
  `CGO_ENABLED=0 go build` cross-compiles (no `fyne package`, no
  platform-specific toolchain needed — `modernc.org/sqlite` was already
  CGO-free before, Fyne was the only CGO dependency in the entire
  module). Verified locally on a single (macOS) machine: all five
  artifacts (`darwin-arm64.app.zip`, `darwin-amd64.app.zip`,
  `windows-amd64.zip`, `linux-amd64.tar.gz`, `linux-arm64.tar.gz`) build
  without errors, `checksums.txt` is produced; the native
  `darwin-arm64` build was additionally actually run (`--help`, `stats`
  against a fresh `HOME`) and works.
- [x] Windows build without a console window (`-H windowsgui`), log to a
  file — `release:windows` links with `-H windowsgui`. Since
  `os.Stderr` writes into the void on a GUI-subsystem build with no
  console window, `internal/platform/logging` was extended with a
  `WithWriter` option and `internal/platform/paths` with
  `LogFilePath()` (`~/Library/Logs/dmarc-analyzer/dmarc-analyzer.log`
  on macOS, otherwise analogous to `ConfigDir()`); `cmd_web.go`
  (`attachLogFile`) additionally writes to this file when starting the
  `web` subcommand (`io.MultiWriter` with `os.Stderr` — in normal
  terminal use, the previous output stays visible unchanged), including
  the login link/address as structured `slog` entries. **Not tested on
  real Windows** (no Windows machine available in this sandbox) — only
  the cross-compile itself (`GOOS=windows GOARCH=amd64`) and the logic
  of the new `logging`/`paths` functions are backed by unit tests.
- [x] macOS: behavior of a non-Cocoa binary inside the `.app` bundle
  checked, as far as possible without a display — `packaging/darwin/
  Info.plist.tmpl` (new, replaces the manifest generated by `fyne
  package`) sets `LSUIElement=true`: the program no longer has its own
  Cocoa event loop (a pure HTTP server process), and without
  `LSUIElement` it would still get a Dock icon, which macOS might mark
  as "not responding". **Not verified in a real graphical session** (no
  display in this development environment) — noted explicitly in the
  Info.plist comment as an open check-point before a real release
  (actual double-click launch, observe Dock behavior by hand).
- [x] Taskfile: `package:*` and the `fyne` install removed, `run` opens
  the browser — `task setup` no longer installs the `fyne` CLI;
  `package:darwin`/`package:windows`/`package:linux`/the old `release`
  replaced by the new `release:*` tasks; `task run` description
  extended ("opens the default browser").
- [x] CI: cross-compile step added — `.github/workflows/ci.yml` already
  existed since the very first commit (`check` job: `task fmt` drift
  check, `lint`, `test`, `build`, as a matrix over
  `ubuntu-latest`/`macos-latest`/`windows-latest`, with pinned versions
  for Go/Task/golangci-lint). Extended with a new `release` job (only
  on a `v*` tag, after `check` succeeds): `task release` (cross-compile
  all target platforms) plus a GitHub release with binaries/checksums
  via `softprops/action-gh-release`, on `macos-latest` (`task release`
  calls `zip`/`shasum` directly — present without extra install on
  macOS, not guaranteed on the Ubuntu runner). **Correction during this
  session:** an early version accidentally overwrote the existing
  `ci.yml` entirely (a wrong assumption that there was no CI yet) — the
  existing `check` job was then restored unchanged and only the new
  `release` job was added.
- [x] Documentation updated per section 12 — `FEATURES.md` (Fyne
  requirement replaced), `IMPLEMENTIERUNG.md` sections 3/5/8.2/10/12/13
  (plus the architecture diagrams in 4.1, which otherwise would have
  contradicted their own update in 5), `UMSETZUNGSPLAN.md` (a reference
  before WP 7, WP 7 items adjusted: browser instead of desktop
  notification, packaging marked as already done in M5),
  `docs/DEPENDENCIES.md` (Fyne/go-chart documented as "Removed",
  htmx/Chart.js/plugins were already recorded before), two new ADRs
  (`docs/adr/0001-web-oberflaeche-statt-fyne.md`,
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`), `AGENTS.md` (all
  six pure Fyne pitfalls removed, the chart-colors section rewritten for
  `app.css`/`charts.js`), `README.md` (fully reworked: actual feature
  state instead of "WP 0", start/browser behavior/quit/
  `--kein-browser`/security model/installation via GitHub releases),
  `CHANGELOG.md` (Changed/Removed/Fixed entries added for the entire
  M0–M5 migration; previously it only had the Fyne state up through
  WP 6).

**Done when:** `go list -deps ./... | grep fyne` is empty, `task check`
is green, and the release binaries for all platforms are produced on
one machine. **Reached** — see the individual items above for details
and the remaining gaps checkable only with a real Windows machine or a
real display (Windows console behavior, macOS Dock behavior).

### M6 — Polish (S–M)

- [ ] Accessibility: full keyboard operation, visible focus, contrast,
  `prefers-reduced-motion`, table views of the charts
- [ ] Visual check in Safari, Firefox, Chrome, and Edge, each in light and dark
- [ ] Extend the smoke test around the write flows, if errors accumulate there

**Done when:** every flow is operable without a mouse and renders
without display errors in all four browsers.

### Order

```
M0 ──▶ M1 ──▶ M2 ──▶ M3 (switchover) ──▶ M4 ──▶ M5 (removal) ──▶ M6 ──▶ WP 7
```

M4 can run in parallel with M3. Fyne stays the default up through and
including M2.

## 11. Tests

| Level | Approach |
| --- | --- |
| Security | `httptest`: no session → 401; wrong `Host` → 421/400; POST without a CSRF token or with a foreign `Origin` → 403; used-up/expired one-time code → rejected; password appears in no response and no log |
| Handlers | `httptest` with the existing hand-written fakes (carried over from `internal/ui/*_test.go`): status codes, redirects (no accounts → first-run setup), rendered core content, empty and error states |
| Templates | Every page is rendered in tests and parsed as HTML — a template error shows up in the test, not first in the browser |
| Chart data | JSON endpoints with `httptest`: empty data, one value, many values; no `NaN`/`Inf` in the JSON (breaks `encoding/json`); drill-down URLs contain the right filters; labels enriched |
| Charts in the browser | `chromedp` smoke test (E-8): four charts drawn, no console errors, one click leads to the filtered report page. `charts.js` stays deliberately thin, so most logic is tested in Go. |
| Lifecycle | Single-instance detection, orphaned `instance.json`, clean shutdown on SIGTERM (open requests finish, `instance.json` removed) |
| Application layer | new tests for the progress callback, `syncjob` (only one run at a time, cancel), `ImportData`, locked key store |
| Visual check | additionally manual per milestone, light and dark |

The existing conventions continue to apply: `testify/require`,
hand-written fakes, `-race`, German comments.

> **Historical note:** "German comments" above reflects the language rule
> in effect at the time this plan was written; the codebase has since
> moved to English throughout (see `AGENTS.md`).

## 12. Documentation

| File | Change |
| --- | --- |
| `FEATURES.md` | replace the Fyne requirement with "embedded web UI in the browser" — **the project's own requirement, please confirm the change** |
| `IMPLEMENTIERUNG.md` | sections 3 (stack), 5 (structure), 8.2 (ChartRenderer), 10 (UI concept instead of Fyne practice), 12 (tests), 13 (Taskfile) |
| `UMSETZUNGSPLAN.md` | reference to this plan before WP 7; adjust WP 7: desktop notification → browser notification while a tab is open, packaging without `fyne package` |
| `docs/DEPENDENCIES.md` | add htmx, Chart.js, `chartjs-chart-matrix`, `chartjs-plugin-zoom` (versions, licenses, compatibility) and `chromedp`; remove Fyne and go-chart |
| `docs/` | ADR "web UI instead of Fyne" and ADR "interactive charts with Chart.js instead of a ChartRenderer port" |
| `AGENTS.md` | remove Fyne sections; new rules: no inline scripts (CSP), every state change via POST with CSRF, always render templates in tests, chart logic (aggregation, filters, drill-down targets) belongs in Go, not in `charts.js` |
| `README.md` | start, browser behavior, quit (Ctrl+C/SIGTERM), `--kein-browser`, security model |
| `CHANGELOG.md` | one entry per milestone |

## 13. Risks

| Risk | Countermeasure |
| --- | --- |
| Server keeps running unnoticed if only the browser tab is closed (desktop use, no Docker) | Deliberately accepted (E-3): no more auto-exit. Mitigated by the console output at startup ("Ctrl+C to quit") and single-instance detection — a second double-click doesn't start another process. If this turns out to be a real annoyance: retrofit in M6, without touching Docker operation (a pure addition, not a replacement for SIGTERM). |
| Other web pages or local users access the server | Section 5 fully in M1, with tests |
| No browser available (server, SSH) | `--kein-browser`, address on the console; CLI stays unchanged |
| Double-click launch behaves unexpectedly per OS (console window, Dock) | Checkpoints in M5 |
| Features get lost in the transfer | Parity table (section 8) as an acceptance list for M3 |
| Chart.js plugins don't fit the integrated Chart.js version or the CSP | Heatmap and zoom first in the spike (M0); pin versions together and only update them together; the ECharts fallback only affects `charts.js` and the JSON shape |
| `<canvas>` is invisible to screen readers | Short description and table view per chart, drill-down also as links |
| A large heatmap (many days) becomes unreadable or slow | Zoom/pan; a default time range; a limit in the smoke test (10 × 365 cells) |
| Logic quietly moves into JavaScript and stays untested | Rule in `AGENTS.md`: preparation and drill-down targets come ready-made from the server |
| The UI will no longer feel "native" in the browser | deliberately accepted |

## 14. Open questions

1. Decisions E-1, E-4 through E-8 — do the recommendations stand? (E-2 is
   fixed with Chart.js, E-3 with Ctrl+C/SIGTERM.)
2. May `FEATURES.md` be changed accordingly?
3. Should the UI language stay German-only (the structure with `i18n`
   would allow English later)?
4. Should the server, on request, also be reachable over the network —
   whether on a home network (NAS) or as a Docker image (section 5,
   "tension with the planned Docker deployment")? This plan only builds
   the loopback model (same machine only); network reachability would
   need a real login (username/password instead of a one-time link) and
   TLS.
5. Is the Docker packaging part of **this** migration (then M5/M6 would
   need to be extended with a `Dockerfile` and the security addendum
   sketched in question 4) or a separate, later plan? Recommendation: a
   separate, later plan — this migration first delivers the local web
   UI; Docker needs a different threat model (question 4) and shouldn't
   be decided as a side effect.
