# Sending sources

`/quellen` aggregates all imported records by source IP
(`internal/domain/sources`, `internal/app/sourcestats`) — volume, pass
rate, time-range filtering, plus enrichment.

## Aggregation

Filterable by time range and domain, sortable by volume (default,
largest source first) or by source IP. Like the report table: keyset
pagination instead of offset, filters live in the URL.

## Enrichment

In addition to the plain report data, every source IP is enriched
(`internal/infra/sourceinfo.Enricher`, PTR/rDNS resolution plus
detection of known sending services, e.g. "Google Workspace"). An
unresolvable PTR or an unrecognized service are normal, expected
outcomes (the UI then shows "—" or the plain IP address), not an error
condition. Results are cached internally — PTR resolution is network
I/O and rarely changes, so a repeated lookup of the same IP isn't worth
it. The same enrichment also feeds into the dashboard's top sending
sources and heatmap charts (the same detected labels).

## CSV export

`GET /export/quellen.csv` exports the entire filtered dataset (not just
the currently displayed page).
