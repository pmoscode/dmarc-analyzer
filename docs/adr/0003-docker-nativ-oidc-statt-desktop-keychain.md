# ADR 0003: Docker-native only with OIDC/Authentik SSO instead of desktop distribution

- Status: accepted
- Date: 2026-09-17

## Context

ADR 0001 replaced the Fyne desktop UI with a web UI embedded in the
program, but implicitly kept the basic model from the desktop era: a
standalone program that runs directly on the user's machine, stores IMAP
credentials in the OS keychain (with an encrypted file fallback for Linux
without a secret service), creates accounts interactively via a
"first-run setup", opens itself via a one-time login link, and listens
only on `127.0.0.1`. This wasn't reconsidered during the move to the web
UI — it predated ADR 0001 and was simply carried over.

In practice it turned out: the web UI opens up the possibility of running
`dmarc-analyzer` as an ordinary Docker container instead, centrally
reachable rather than installed on a single workstation. That makes
several parts of the previous security and operations model moot:

- An OS keychain makes no sense in a container — there's no "user
  desktop" whose keychain could be opened. IMAP credentials can instead
  be passed in more simply and in the container-conventional way, via
  environment variable (12-factor).
- Platform-appropriate paths (`os.UserConfigDir()` per OS), browser
  auto-open, and file-based single-instance detection are pure desktop
  concepts — a container has a fixed data directory (volume) and only
  ever runs once per container anyway.
- A centrally reachable service instead of "runs only on 127.0.0.1, one
  user per machine" needs real multi-user access control. The previous
  one-time login link (printed to the console) assumes the console and
  browser are on the same device — that no longer holds for a server.
  An Authentik IdP already exists to authenticate against.

## Decision

1. **IMAP credentials and all runtime settings come exclusively from
   environment variables** (`internal/infra/envconfig`, see
   `docs/features/deployment.md` for the full reference). Exactly one
   account per container — no more account CRUD in the UI, no first-run
   wizard. For multiple mailboxes: multiple containers, each with its
   own database.
2. **Access protection via OIDC against the existing Authentik IdP**
   (authorization code flow with PKCE, `internal/web/oidc.go`) instead of
   a one-time login link. Access only for members of a configurable
   Authentik group (`DMARC_OIDC_ADMIN_GROUP`, checked against the
   `groups` claim of the ID token). Sessions are now real, concurrently
   running multi-user sessions (`internal/web/auth.go`), no longer a
   single global session per process.
3. **The server no longer binds exclusively to loopback addresses** —
   instead, the Host header is checked against the public hostname
   derived from `DMARC_OIDC_REDIRECT_URL` (`requireHost`,
   `internal/web/middleware.go`); access protection now comes from OIDC,
   no longer from "only reachable from the same machine".
4. **Delivered as a Docker image** (`Dockerfile`, multi-stage,
   `CGO_ENABLED=0`, `gcr.io/distroless/static-debian12:nonroot` as the
   runtime base) instead of platform-specific binaries (`.app` bundle,
   `.exe`, `.tar.gz`) as GitHub release assets. CI now builds only on
   `ubuntu-latest` (see `.github/workflows/ci.yml`) and publishes an
   image to GHCR on a version tag instead of building release binaries
   for three platforms.

Removed: `internal/infra/keyring` (OS keychain + file fallback),
`internal/platform/paths` (platform-appropriate paths),
`internal/web/browser.go` (browser auto-open), `internal/web/instance.go`
(single-instance detection via `instance.json`),
`internal/web/handlers_onboarding.go` (first-run wizard),
`internal/web/handlers_unlock.go` (keychain unlock screen),
`packaging/darwin/` (`.app` bundle metadata).

## Consequences

**Advantages:**

- Noticeably simpler operations model: one container, one ENV file, one
  volume — no more "where does the database live on this OS", no more
  keychain-fallback question catalog.
- A real multi-user role model instead of "one user per installed
  program" — fits a centrally operated service.
- CI/release noticeably leaner (one runner OS instead of three, one
  artifact type instead of four).

**Disadvantages / deliberately accepted:**

- No more offline operation without a working OIDC provider — deliberately
  accepted, because the use case (centrally operated team tool with an
  existing Authentik) requires it.
- Anyone who still wants to run `dmarc-analyzer` as a pure single-user
  tool without Authentik would need to set up their own minimal OIDC
  provider, or fall back to a version predating this ADR — that use case
  is no longer actively supported.
- No more `docker exec`-less CLI experience like before (`account add`
  etc.) — accounts now come only from ENV, which trades interactivity
  for automatability.

## Alternatives

- **Forward-auth/outpost in front of the app instead of its own OIDC
  client:** Authentik as reverse-proxy middleware (e.g. Traefik
  ForwardAuth) would have freed the app itself from any auth logic.
  Rejected in favor of an app-native OIDC client, so the app stays
  independent of the specific reverse proxy and doesn't have to run
  behind a particular outpost setup.
- **Retention/sync interval still via a web UI form:** rejected in favor
  of consistently pure ENV configuration (12-factor) — otherwise there
  would be two different configuration paths side by side.
