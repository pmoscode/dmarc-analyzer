# IMAP fetching and import

## Incremental sync

`internal/app/syncreports.UseCase.SyncAccount` connects to the configured
IMAP mailbox (`internal/infra/imap`) and fetches **only new** messages
since the last run (`sync.State`: last-seen UID + UIDVALIDITY). If the
mailbox's UIDVALIDITY changes (e.g. after a provider-side mailbox
migration), the protocol detects that itself and a full rescan runs
again — the `UNIQUE` index on `reports` (`org_name`, `report_id`,
`date_begin`) prevents duplicates from that.

Fetching and parsing run concurrently, writing to SQLite is serial —
details in
[`../architecture.md`](../architecture.md#sync-pipeline-internalappsyncreports).

A sync run is triggered:

- **manually** via the sync button in the web UI (`POST /abgleich`),
  with live progress via Server-Sent Events (`GET /ereignisse`);
- **automatically** every `DMARC_SYNC_INTERVAL_MINUTES` minutes in the
  background (`internal/app/syncscheduler`, see
  [`deployment.md`](deployment.md));
- **via `docker exec`** (`dmarc-analyzer sync`) for diagnostics/maintenance.

At most one run at a time — if a sync is already running, another start
attempt is a no-op, not an error.

## Supported formats

DMARC aggregate reports (RUA, RFC 7489) arrive as an email attachment in
various packagings. Supported are `.xml`, `.xml.gz`, and `.zip` (including
**multiple** XML files inside — each is imported as its own report, see
`internal/infra/dmarcxml`'s `ParseAll`/`domainsync.MultiReportParser`). An
attachment that no registered `sync.ReportParser` recognizes (e.g. the
message's own text body or a company logo attachment) is silently
skipped — not an error.

Unknown or RFC-deviating enum values in the XML (providers don't always
follow RFC 7489 exactly) are never discarded, but mapped to an `Unknown`
value — a single deviating field must not block the import of the entire
report.

## Error quarantine

If a message or attachment can't be processed (broken XML, unexpected
format), that does **not** abort the entire sync run — the error lands in
the error quarantine (the `failed_imports` table,
`domainsync.FailedImportRepository`) along with the unprocessed raw
bytes, so the message can be re-read after a parser fix without querying
the mailbox again.

## File import without IMAP

Reports can also be imported directly via the web UI (`/import`, drag &
drop or file selection) or via `docker exec dmarc-analyzer import
<path>...` — without IMAP credentials, for offline operation or migrating
old data (`internal/app/importfiles`). Shares MIME splitting and parsers
with the IMAP sync, but doesn't implement its own `sync.MessageSource` —
a local file import has no IMAP credentials, so going through that
interface would be forced rather than natural.
