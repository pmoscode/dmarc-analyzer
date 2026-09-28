# ADR 0002: Chart.js in the browser instead of an `analysis.ChartRenderer` port

- Status: accepted
- Date: 2026-09-17 (migration decision), implemented in `MIGRATIONSPLAN.md`
  M0 (spike) through M5 (port removed)

## Context

Before the web migration, `internal/domain/analysis.ChartRenderer`
provided a port that `internal/infra/charts` implemented against
[`go-chart/v2`](https://github.com/wcharczuk/go-chart) (`IMPLEMENTIERUNG.md`
section 8.2: "ChartRenderer as a port", intended as a swappable adapter —
e.g. later against native, interactive Fyne widgets). In practice:

- Charts were server-rendered `image.Image`/PNGs with no tooltip, no
  showing/hiding data series, no zoom, no click drill-down.
- `go-chart` has no heatmap type — `internal/infra/charts/heatmap.go`
  drew the source-×-day matrix manually via `image/draw`.
- The PNGs had a fixed white background (`flattenOnWhite`), because
  `go-chart` offered no transparency for the surrounding card
  background — in dark system mode this showed up as bright boxes
  around every chart.
- `internal/infra/charts/renderer.go` contained several pure workarounds
  for go-chart quirks (text wrapping, single-value donut, transparent
  borders) with no business value.

With the move to a web UI (ADR 0001), the original motivation for the
port ("swap it later for native, interactive Fyne widgets") goes away: in
the browser, charts are inherently client-side, not server-rendered
images.

## Decision

Charts are produced entirely in the browser with
[Chart.js](https://www.chartjs.org/) (`internal/web/static/charts.js`),
augmented with the plugins `chartjs-chart-matrix` (the heatmap chart
type, which Chart.js itself doesn't include) and `chartjs-plugin-zoom`
(zoom/pan in the time series). The server delivers only prepared JSON
data via its own endpoints (`/api/diagramme/*`); all aggregation,
filter, and drill-down decisions are made in Go
(`internal/app/statistics`, `internal/domain/analysis`), and `charts.js`
stays deliberately thin (pure presentation, see `AGENTS.md`: "Chart logic
belongs in Go").

This entirely removes:

- `analysis.ChartRenderer` (port, `internal/domain/analysis/charts.go`) —
  the plain data types `DailyVolume`, `SourceVolume`, `Heatmap`,
  `HeatmapCell` remain unchanged and are now serialized directly to JSON
  instead of being handed to an image renderer. `HeatmapCell` gained an
  additional `Total` field for this in M2 (message count per cell, for
  the browser tooltips).
- `internal/infra/charts` (the go-chart implementation of the port, 4
  files including the manual heatmap drawing).
- `exportdata.WriteChartPNG` (PNG-encoding a rendered chart image — its
  only caller was the now-deleted Fyne dashboard PNG export in
  `internal/ui/dashboard/view.go`).
- `github.com/wcharczuk/go-chart/v2` from `go.mod`.

Chart export as a file (PNG/CSV) remains as a feature, but is now purely
client-side: `chart.toBase64Image()` for PNG, a small JavaScript object
built from the same data as the associated table view for CSV (see
`internal/web/static/charts.js`, the `chartExports` registry) — no
server-side image-encoding step needed anymore.

A considered alternative was [Apache ECharts](https://echarts.apache.org/):
it already comes with heatmap, zoom, and image export built in, but is
noticeably larger at ≈1 MB and has its own theming system that would have
needed to be reconciled with the existing CSS-variable palette
(`AGENTS.md`: "Chart colors follow the `dataviz` skill's reference
palette") (`MIGRATIONSPLAN.md` decision E-2). Chart.js plus two small,
targeted plugins was chosen as the leaner solution; ECharts remains the
documented fallback should the plugin combination prove untenable — in
that case only `charts.js` and the JSON shape of the endpoints would be
affected, not the Go side.

## Consequences

**Advantages:**

- Real interactivity: tooltips, toggleable legend, zoom/pan in the time
  series, click drill-down on day/source/heatmap cell/disposition — each
  leading to a filtered report view (the server supplies the target URLs
  ready-made).
- Charts automatically adapt to light/dark (Chart.js reads the colors
  from the same CSS variables as the rest of the UI, see the `AGENTS.md`
  section on the chart color palette) — no more fixed white background.
- Noticeably less code and no pure library workarounds (all the
  go-chart workarounds disappeared along with the package).
- A smaller, CGO-free server build (see ADR 0001) — the charting library
  now lives as a minified JavaScript file in the repository
  (`internal/web/static/vendor/`, see `docs/DEPENDENCIES.md`), no longer
  as a Go dependency.

**Disadvantages / deliberately accepted:**

- A `<canvas>` is invisible to screen readers — mitigated by a short
  description (`aria-label`) and a toggleable table view per chart
  (`<details><summary>Show as table</summary>...`); drill-down targets
  are also available as regular links in that table.
- With Chart.js, real logic lives in JavaScript (data mapping, colors,
  click targets) that pure `httptest` handler tests can't see — for this,
  `MIGRATIONSPLAN.md` section 11 plans a `chromedp` smoke test (E-8)
  (four charts drawn, no console errors, one drill-down click works). In
  the development environments where M2–M5 were built, no
  Chrome/Chromium was available — the smoke test remains unwritten so
  far and is noted as an open item in `MIGRATIONSPLAN.md`, not part of
  this decision itself.
- A pitfall with `responsive: true`/`maintainAspectRatio: false` that
  had already come up once and stayed undocumented (a canvas with no
  wrapper of fixed height grows unbounded) is now recorded in
  `AGENTS.md` — a pure Chart.js API trap, not a fundamental argument
  against the decision.

## Alternatives

- **Keep the `analysis.ChartRenderer` port, just embed charts in the
  browser via an image tag:** wouldn't have achieved the actual
  benefits (tooltips, zoom, click drill-down, automatic light/dark
  adaptation) — would just have been a PNG in an `<img>` tag instead of
  in the Fyne window.
- **Apache ECharts:** see above, documented as a fallback, not chosen
  (size, its own theming system).
- **D3.js (full control, no ready-made charting framework):** not
  seriously considered — noticeably more code for the same four chart
  types, without the maturity/tooltip/zoom building blocks Chart.js
  already provides out of the box.
