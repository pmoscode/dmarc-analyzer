# Overview (dashboard)

`/` shows the central metrics for a selectable time range
(`internal/app/statistics.UseCase.Dashboard`), filtered by time range and
domain (filter bar, values live in the URL).

## Metrics

- **Total messages** — sum of all `Record.Count` in the time range (a
  record with Count=50 is 50 messages from that source, not one report).
- **DMARC pass rate** — share of messages that are DKIM- or SPF-aligned.
- **DKIM/SPF alignment rate** — reported separately, because a message
  stream with a high pass rate can still have a weak alignment rate for
  exactly one of the two mechanisms (relevant for troubleshooting).
- **Number of distinct source IPs**.
- **Trend vs. the previous period** — pass rate of the current period
  minus the pass rate of the immediately preceding period of equal
  length.

## Charts

All four charts load their data themselves via JavaScript from
`/api/diagramme/*` (JSON, prepared in Go — `charts.js` stays deliberately
thin) and respond to light/dark mode via CSS variables:

- **Message volume per day**, stacked by pass/fail, with zoom.
- **Distribution by disposition** (none/quarantine/reject).
- **Top sending sources by volume**, colored by pass rate.
- **Source × day** as a heatmap, color = pass rate, tooltip with
  source, day, pass rate, and message count.

Every chart element (day, source, cell, segment) is clickable and opens
the report table with a correspondingly preset filter (drill-down).
Individual charts can also be exported directly as PNG or CSV.
