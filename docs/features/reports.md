# Reports

## Storage

Every imported DMARC aggregate report is stored fully or not at all
(`report.Repository.Save` runs in a transaction): metadata (sending
organization, report period, published policy) plus all contained
records (source IP, message count, disposition, DKIM/SPF results,
reasons), as well as the complete raw report (for later traceability).
Deduplication via a `UNIQUE` index on (`org_name`, `report_id`,
`date_begin`) — the same report imported via two different paths (e.g.
via IMAP and also via file upload) is stored only once.

## Report table (`/berichte`)

Filters, sorts, and groups server-side in SQL (not offset but keyset
pagination — stays fast even with many reports):

- **Filter**: time range, domain, sending organization, source IP,
  disposition.
- **Sort**: by report start, sending organization, or domain.
- **Group** (acts as an additional, primary sort key): by domain, by
  organization, or by source IP.

Filters live in the URL — bookmarks and the browser's back button work.

## Report detail (`/berichte/{id}`)

Loads a single report fully, including all records — the report table
itself deliberately doesn't load records (one row per report, not per
record).

## CSV export

`GET /export/berichte.csv` exports the **entire filtered dataset** (not
just the currently displayed page) as CSV (`internal/app/exportdata`).
Individual dashboard charts can also be exported directly as PNG or CSV.
