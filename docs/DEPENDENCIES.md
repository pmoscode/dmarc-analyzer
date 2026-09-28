# Dependencies

> Complements `docs/architecture.md`. That file has the *rationale* for
> each library in broad strokes; this one has the *concretely pinned
> version*.

## Principle

A dependency is only added to `go.mod` via `go get <module>@<version>`
once code actually imports it. A `go.mod` that contains dependencies
imported nowhere lies about the actual state and gets removed again by
the next `task tidy` anyway.

## In use

| Module                            | Version         | Usage                                                                                                                                                  |
|------------------------------------|-----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| `github.com/stretchr/testify`    | `v1.12.1`       | `require` in all tests                                                                                                                                 |
| `modernc.org/sqlite`             | `v1.59.0`       | Persistence (`internal/infra/sqlite`), CGO-free — required for `CGO_ENABLED=0` in the Docker build                                                    |
| `github.com/emersion/go-imap/v2` | `v2.0.0-beta.8` | IMAP adapter (`internal/infra/imap`)                                                                                                                   |
| `github.com/emersion/go-message` | `v0.18.2`       | MIME splitting (`internal/infra/mailmime`) into `sync.RawAttachment`                                                                                   |
| `github.com/coreos/go-oidc/v3`   | `v3.21.0`       | OIDC client (discovery, ID token validation) against Authentik, see `docs/features/auth.md`                                                           |
| `golang.org/x/oauth2`            | `v0.37.0`       | Authorization code flow + PKCE for the OIDC login                                                                                                      |
| `github.com/go-jose/go-jose/v4`  | `v4.1.5`        | Transitive via `go-oidc` (JWT/JWS signature verification), directly imported in the test double `internal/web/fakeoidc_test.go` (local fake identity provider) |

### Frontend (`internal/web/static/vendor/`) — not a Go module, minified files checked into the repository

Instead of a CDN, htmx, Chart.js and its plugins are kept as minified UMD
bundles directly in the repository (works offline, fits the Content
Security Policy `default-src 'self'`). Licenses sit alongside as
`LICENSE.<name>.txt` each.

| File                           | Version  | License      | Usage                                                                |
|---------------------------------|----------|--------------|------------------------------------------------------------------------|
| `chart.umd.min.js`            | `4.5.1`  | MIT          | Charts in the browser (`internal/web/static/charts.js`)                |
| `chartjs-chart-matrix.min.js` | `3.1.0`  | MIT          | Heatmap chart type (`matrix`)                                          |
| `chartjs-plugin-zoom.min.js`  | `2.2.0`  | MIT          | Zoom/pan in the time series                                            |
| `htmx.min.js`                 | `2.0.10` | BSD-0-Clause | Server-rendered partial updates (pagination, dialogs)                  |

On a version update: update all three Chart.js files (core + both
plugins) together and test them against each other (plugins don't
necessarily follow the same version scheme as Chart.js itself).

The standard library (`encoding/xml`, `compress/gzip`, `archive/zip`,
`database/sql`, `log/slog`, `net/http`) needs no pinning.

## Removed

- `fyne.io/fyne/v2` and `github.com/wcharczuk/go-chart/v2` — removed with
  the move from the Fyne desktop UI to the embedded web UI. Rationale:
  `docs/adr/0001-web-oberflaeche-statt-fyne.md`,
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.
- `github.com/zalando/go-keyring`, `golang.org/x/crypto` (scrypt),
  `github.com/danieljoos/wincred`, `github.com/godbus/dbus/v5` — the OS
  keychain adapter (`internal/infra/keyring`) went away with the move to
  pure ENV configuration; IMAP credentials now come from
  `DMARC_IMAP_PASSWORD` instead of a persisted keychain. Rationale:
  `docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`.
- `github.com/google/uuid` — was only needed for freely assigned account
  IDs when interactively creating an account; since the ENV configuration
  model there's only ever one account with a fixed ID per container.
