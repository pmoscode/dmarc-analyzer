# Retention policy

DMARC aggregate reports aren't kept forever — after a configurable
number of months they're automatically deleted.

## Configuration

`DMARC_RETENTION_MONTHS` (default: `24`, see
[`deployment.md`](deployment.md)). `0` means: unlimited retention, no
automatic deletion. A change requires a container restart (12-factor: no
runtime form for this, see
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`).

The currently effective values (retention period and sync interval) are
shown read-only on the status page (`/einstellungen`).

## When a report counts as "old"

A report counts as old as soon as its report period ends entirely before
the cutoff date `today − DMARC_RETENTION_MONTHS months` — what matters is
the end of the period stated in the report itself (`date_end`), not the
time it was imported.

## When the rule is applied

`internal/app/retentionjob.Runner` applies the rule automatically in the
background: once immediately at container startup (a freshly changed
retention period shouldn't only take effect after a full day) and then
every 24 hours, for the lifetime of the container.

## Implementation

Deletion goes through `report.Pruner.DeleteOlderThan` — a narrow, own
port (`internal/domain/report/repository.go`), separate from
`report.Repository`, because deleting by age is a pure maintenance
operation. The SQLite implementation
(`internal/infra/sqlite.ReportRepository.DeleteOlderThan`) deletes via a
single `DELETE FROM reports WHERE date_end < ?` — the associated records,
auth results, reasons, and the raw report are automatically removed too,
via existing `ON DELETE CASCADE` foreign keys.
