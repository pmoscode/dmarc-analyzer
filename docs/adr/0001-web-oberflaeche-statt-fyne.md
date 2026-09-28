# ADR 0001: Embedded web UI instead of Fyne

- Status: accepted
- Date: 2026-09-17 (migration decision), implemented in `MIGRATIONSPLAN.md`
  M0–M5 (Fyne fully removed in M5)

## Context

`dmarc-analyzer` was originally a desktop program with
[Fyne](https://fyne.io/) as its UI toolkit (`FEATURES.md` explicitly named
Fyne as a requirement, `IMPLEMENTIERUNG.md` section 1.2 treated that as
binding). In practice, several problems showed up:

- Despite repeated rework (custom theme, color palette, card layout), the
  UI still felt heavy/"clunky".
- Fyne is the **only CGO dependency** of the entire module — checked on
  2026-09-17: without `internal/ui`, all packages already build with
  `CGO_ENABLED=0` for `linux/amd64`, `windows/amd64`, and `darwin/arm64`.
  Cross-compiled releases without native toolchains per target platform
  weren't readily possible with Fyne.
- Fyne v2.8 offers no real tooltips (workaround: a custom "?" button next
  to every chart/metric, see the by-now-deleted `internal/ui/glossary`
  references).
- Charts were produced server-side as static PNGs (`go-chart`,
  `internal/infra/charts`) — no tooltip, no zoom, no click drill-down, a
  fixed white background (`flattenOnWhite`) even in dark system mode.
- Over time, `internal/ui` accumulated several Fyne-specific pitfalls of
  its own (documented and then removed from `AGENTS.md` along with the
  package: `TestMain`/`test.NewApp()`, `runBackground` injection for
  data-race-free tests, `container.NewBorder` index ordering,
  `widget.Select.SetSelected()` callback timing, `uitest.FindEntries`
  type assertion, `dialog.NewCustomWithoutButtons`). Each of these traps
  cost time and was pure Fyne API knowledge with no business value.

## Decision

The entire presentation layer is replaced by a **web UI embedded in the
program**: on startup, a local HTTP server runs (`internal/web`, `net/http`
+ `html/template` from the standard library, no bundler, no Node
tooling), and the default browser automatically opens the main page. A
considered alternative was an SPA (Svelte/Vue/React) with its own Vite
build — rejected because it would need a second tooling ecosystem and its
own tests, while server-side rendering is plenty for tables and forms
(`MIGRATIONSPLAN.md` decision E-1).

The migration ran in milestones (`MIGRATIONSPLAN.md` M0–M5): first build
`internal/web` alongside `internal/ui` (decision E-5), switch over once
feature parity is reached (M3: `dmarc-analyzer` with no arguments starts
the web UI instead of Fyne), then in M5 fully remove `internal/ui`,
`cmd/dmarc-analyzer/cmd_gui.go`, and `fyne.io/fyne/v2` (along with all
transitive Fyne dependencies).

The lifecycle deliberately follows the server model, not the desktop-app
model: only Ctrl+C/SIGTERM end the process (no "quit" button, no
auto-exit after idle, no tray icon) — see `MIGRATIONSPLAN.md` decision
E-3 and the associated risk entry in section 13 ("server keeps running
unnoticed if only the browser tab is closed" — a deliberately accepted
tradeoff).

## Consequences

**Advantages:**

- No more CGO dependency anywhere in the module; `task release` builds
  all target platforms (macOS arm64/amd64, Windows amd64, Linux
  amd64/arm64) via a plain `CGO_ENABLED=0 go build ...` on a single
  machine, without platform-specific toolchains.
- Interactive charts with tooltips, a toggleable legend, zoom, and click
  drill-down (see ADR 0002) instead of static PNGs.
- Charts automatically adapt to light/dark (Chart.js reads CSS
  variables).
- Filters live in the URL — bookmarks and the back button work.
- All Fyne-specific test traps (see above) disappeared along with the
  package; the web layer has its own, but more general, pitfalls
  (documented in `AGENTS.md`, e.g. CSP/CSRF/Chart.js resize).

**Disadvantages / deliberately accepted:**

- The UI no longer feels "native" (no native window look-and-feel) —
  deliberately accepted, see the `MIGRATIONSPLAN.md` risk table.
- A server on `127.0.0.1` isn't automatically private (other users of
  the same machine, DNS rebinding, CSRF from other open web pages) —
  needs its own security model (one-time login link, session cookie,
  `Host` check, CSRF token, CSP), implemented in M1 and justified in
  detail in `MIGRATIONSPLAN.md` section 5. A pure desktop program with
  its own window wouldn't have had this problem.
- Without its own display in the development environment where M4/M5
  were built, some behaviors (browser smoke test, macOS dock behavior of
  the `.app` bundle) couldn't be conclusively verified visually —
  documented at the respective spots (`MIGRATIONSPLAN.md`,
  `packaging/darwin/Info.plist.tmpl`).

## Alternatives

- **Keep Fyne, just improve charts/tooltips:** wouldn't have solved the
  CGO problem or the "clunky" feel.
- **SPA with its own frontend build (Svelte/Vue/React):** rejected, see
  decision E-1 above — disproportionate extra effort for a
  one-person/small-team tool that's mostly tables/forms.
- **Electron/Tauri (web tech in its own window):** not seriously
  considered — brings back its own runtime/bundling problem (with
  Electron, Chromium shipped as well), without keeping the advantage of
  "already runs in the user's browser, no extra runtime".
