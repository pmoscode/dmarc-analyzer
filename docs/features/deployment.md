# Deployment (Docker)

`dmarc-analyzer` runs exclusively as a Docker container, fully configured
through environment variables (12-factor) — see
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md` for the
rationale behind this move.

## Quick start

```sh
docker run -d \
  --name dmarc-analyzer \
  -p 8080:8080 \
  -v dmarc-data:/data \
  --env-file .env \
  ghcr.io/pmoscode/dmarc-analyzer:latest
```

A complete example configuration including all variables lives in
[`.env.example`](../../.env.example) in the repository root — copy it to
`.env` and fill it in; [`docker-compose.yml`](../../docker-compose.yml)
reads that file via `env_file`.

To build locally instead of using the published image:
`task docker:build`, or with Compose `task compose:build`/`task
compose:up` (Taskfile.yml). The tasks pass the version (`git describe`)
and Git commit as build args `VERSION`/`COMMIT`; both show up in the UI
under the wordmark, in the startup log, and as OCI labels in the image
(`task docker:version`). A direct `docker build -t dmarc-analyzer .` or
`docker compose build` works too, but then only shows `dev` without a
commit — unless `VERSION`/`COMMIT` are set themselves (`--build-arg` or
exported).

## Environment variables

### IMAP account (required)

Exactly one account per container. For multiple mailboxes: multiple
containers, each with its own `/data` volume.

| Variable               | Required | Default | Meaning                                                                    |
|-------------------------|----------|---------|-------------------------------------------------------------------------------|
| `DMARC_IMAP_HOST`     | yes      | —       | IMAP host, e.g. `imap.example.com`                                        |
| `DMARC_IMAP_PORT`     | no       | `993`   | IMAP port                                                                  |
| `DMARC_IMAP_USER`     | yes      | —       | Username/email address                                                    |
| `DMARC_IMAP_PASSWORD` | yes      | —       | Password/app password — never build into an image, set only at runtime   |
| `DMARC_IMAP_MAILBOX`  | no       | `INBOX` | Mailbox reports are fetched from                                          |
| `DMARC_IMAP_TLS`      | no       | `true`  | Use IMAPS — set `false` only deliberately (plaintext IMAP)                |

### Retention and background sync

| Variable                       | Required | Default | Meaning                                                                                             |
|----------------------------------|----------|---------|---------------------------------------------------------------------------------------------------------|
| `DMARC_RETENTION_MONTHS`      | no       | `24`    | Reports older than N months are automatically deleted. `0` = unlimited                              |
| `DMARC_SYNC_INTERVAL_MINUTES` | no       | `60`    | Interval between automatic background syncs. `0` = manual only, via the sync button                |

See [`retention.md`](retention.md) for details on the retention policy.

### Login (OIDC/Authentik, required)

See [`auth.md`](auth.md) for the complete login flow and the Authentik
setup.

| Variable                    | Required | Meaning                                                                                                   |
|-------------------------------|----------|-----------------------------------------------------------------------------------------------------------|
| `DMARC_OIDC_ISSUER_URL`    | yes      | Authentik application URL, e.g. `https://authentik.example.com/application/o/dmarc/`                     |
| `DMARC_OIDC_CLIENT_ID`     | yes      | Client ID of the Authentik application                                                                    |
| `DMARC_OIDC_CLIENT_SECRET` | yes      | Client secret of the Authentik application                                                                |
| `DMARC_OIDC_REDIRECT_URL`  | yes      | Full, publicly reachable callback URL, e.g. `https://dmarc.example.com/anmelden/callback`                 |
| `DMARC_OIDC_ADMIN_GROUP`   | yes      | Authentik group name that grants access (checked against the `groups` claim)                              |
| `DMARC_OIDC_INSECURE_SKIP_VERIFY` | no | Disables TLS certificate verification for all calls to the issuer (discovery, JWKS, token exchange), default `false`. **Only for development environments with a self-signed certificate** (e.g. Caddy's `tls internal`) — never set this in production, or the token exchange and ID token validation are unprotected against a man in the middle. |

### Operations

| Variable            | Required | Default | Meaning                                                                                                                                      |
|----------------------|----------|---------|----------------------------------------------------------------------------------------------------------------------------------------------|
| `DMARC_DATA_DIR`    | no       | `/data` | Directory for the SQLite database — mount as a volume                                                                                       |
| `DMARC_LISTEN_ADDR` | no       | `:8080` | Address the HTTP server binds to                                                                                                             |
| `DMARC_DEV_MODE`    | no       | `false` | Load templates/static assets from disk instead of embedded (local development only, working directory must be the repository root)          |

If a required variable is missing or a value is invalid, the process
aborts at startup with an error message that lists **all** problems at
once (`internal/infra/envconfig`) — not just the first one, so you don't
have to restart repeatedly to discover each missing variable one at a
time.

## Healthcheck

`GET /gesund` responds unauthenticated with `200 OK` as long as the HTTP
server is running (no database access — a single slow query shouldn't
incorrectly mark the container as "unhealthy"). The Docker image has no
`curl`/`wget` (distroless base) — the Dockerfile instead uses
`dmarc-analyzer healthcheck`, its own subcommand of the same binary, as
the `HEALTHCHECK`.

## Persistence

The SQLite database lives at `$DMARC_DATA_DIR/dmarc.db`. This directory
must be mounted as a volume — without a volume, all imported reports are
lost when the container is removed.

## Diagnostics via `docker exec`

Besides the web server (`web`, also the default with no arguments), the
binary comes with additional subcommands for maintenance/debugging:

```sh
docker exec dmarc-analyzer /dmarc-analyzer stats --days 30
docker exec dmarc-analyzer /dmarc-analyzer sync

# File import: copy into the container first, then import
# (or directly via the web UI at /import via drag & drop).
docker cp report.xml dmarc-analyzer:/tmp/report.xml
docker exec dmarc-analyzer /dmarc-analyzer import /tmp/report.xml
```
