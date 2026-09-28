# DMARC Analyzer

A program that runs as a Docker container, fetches DMARC aggregate reports
(RUA) from an IMAP mailbox, stores them in SQLite, and evaluates them via an
embedded web UI. Access is protected via OIDC against an existing Authentik
identity provider, and everything is configured through environment
variables.

> Architecture: [`docs/architecture.md`](docs/architecture.md). Per-domain
> feature docs: [`docs/features/`](docs/features/). Architecture decisions
> with rationale: [`docs/adr/`](docs/adr/). `docs/archive/` contains the
> planning documents from the earlier desktop era — historical context, not
> the current state.

## What the program does

- Connects to a configured IMAP mailbox and fetches **only new** DMARC
  aggregate reports incrementally (`.xml`, `.xml.gz`, `.zip`, including
  attachments with multiple reports) — see
  [`docs/features/imap-sync.md`](docs/features/imap-sync.md).
- Reports can also be imported directly via drag & drop or file selection in
  the web UI (`/import`), without IMAP access.
- Shows key metrics on an overview page (total messages, DMARC pass rate,
  DKIM/SPF alignment rate, number of distinct sources) and four interactive
  charts (Chart.js: time series, disposition, top sending sources,
  source-×-day heatmap) with drill-down into the report table — see
  [`docs/features/dashboard.md`](docs/features/dashboard.md).
- Filters, sorts, and groups reports and sending sources by time range,
  domain, sending organization, and source IP; filters are reflected in the
  URL (bookmarks and the back button work) — see
  [`docs/features/reports.md`](docs/features/reports.md) and
  [`docs/features/sources.md`](docs/features/sources.md).
- Exports reports and sending sources as CSV (the whole filtered set) as
  well as individual charts as PNG or CSV.
- Automatically deletes reports after a configurable retention period —
  see [`docs/features/retention.md`](docs/features/retention.md).
- Access protection via OIDC/Authentik, restricted to members of a
  configurable group — see [`docs/features/auth.md`](docs/features/auth.md).

## Quick start

```sh
docker run -d \
  --name dmarc-analyzer \
  -p 8080:8080 \
  -v dmarc-data:/data \
  --env-file .env \
  ghcr.io/pmoscode/dmarc-analyzer:latest
```

Full environment variable reference: [`docs/features/deployment.md`](docs/features/deployment.md);
an example with all values lives in [`.env.example`](.env.example) (copy to
`.env` and fill it in — read in via `env_file` by
[`docker-compose.yml`](docker-compose.yml)).

Requires an existing Authentik identity provider — setup instructions in
[`docs/features/auth.md`](docs/features/auth.md).

## Development setup

Requirements: Go 1.27+, [Task](https://taskfile.dev/), Docker (for
`task docker:*`/`compose:*`).

```sh
task setup         # install tools, download dependencies
task check         # fmt + lint + test — before every commit; also what CI runs
task run           # start the program in development mode (web UI)
task build         # build the binary into bin/
task docker:build  # build the Docker image locally (with version and Git commit)
task compose:up    # build via docker compose and start in the background
```

All available tasks: `task --list`.

## License

[MIT](LICENSE)
