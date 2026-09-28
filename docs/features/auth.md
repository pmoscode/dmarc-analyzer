# Login (OIDC/Authentik)

Access to the web UI is secured via OpenID Connect against an existing
[Authentik](https://goauthentik.io/) identity provider — only members of
a configurable Authentik group are allowed to see the application.
Rationale for this model (instead of a one-time login link like in the
earlier desktop version):
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`.

## Flow (authorization code flow + PKCE)

1. `GET /anmelden` — creates a short-lived login attempt (`state`,
   `nonce`, PKCE verifier), stores it server-side, and redirects to
   Authentik's authorize endpoint (discovered via OIDC discovery from
   `DMARC_OIDC_ISSUER_URL`).
2. Authentik authenticates the user and redirects back to
   `DMARC_OIDC_REDIRECT_URL` (must be exactly
   `<public-URL>/anmelden/callback` and must be entered as the redirect
   URI in Authentik).
3. `GET /anmelden/callback` — exchanges the code for tokens, validates
   the ID token (issuer, audience, signature, expiry, nonce), and checks
   whether the `groups` claim contains `DMARC_OIDC_ADMIN_GROUP`. If group
   membership is missing, this returns `403 Forbidden` instead of a
   session — no automatic re-redirect to Authentik (that would loop
   forever for a permanently missing authorization).
4. On success, a session is created (cookie `dmarc_session`, valid for
   12h) and the user is redirected to the overview page.
5. `POST /abmelden` ends the session and — if Authentik's discovery
   document advertises an `end_session_endpoint` — redirects there
   (RP-initiated logout), otherwise to `/anmelden`. For this, the
   Content Security Policy (`internal/web/middleware.go`) allows the
   origin of `DMARC_OIDC_ISSUER_URL` in `form-action` in addition to
   `'self'` — without that, the browser would block the redirect to a
   different origin; the local session would be ended, but Authentik
   would never learn about the logout.

Several admins can be logged in at once from different browsers — each
session is independent (`internal/web/auth.go`).

## Authentik setup

1. Create a new OAuth2/OpenID provider application in Authentik.
2. Set the redirect URI to exactly `<public-URL>/anmelden/callback`.
3. Scopes: at least `openid`, `profile`, `email`, **`groups`** (the
   `groups` claim in the ID token is required for the access check —
   without this scope, the claim stays empty and no one can log in).
4. Create a group (or use an existing one), enter its name as
   `DMARC_OIDC_ADMIN_GROUP`, and assign the authorized users to that
   group.
5. Copy the client ID/client secret from Authentik into
   `DMARC_OIDC_CLIENT_ID`/`DMARC_OIDC_CLIENT_SECRET`.

## Network access protection

The server additionally checks the `Host` header of every request
against the public hostname derived from `DMARC_OIDC_REDIRECT_URL`
(`requireHost`, `internal/web/middleware.go`) — a reverse proxy must pass
the external Host header through unchanged, otherwise the server
responds with `421 Misdirected Request`.

Cookies are set with `Secure`+`HttpOnly`+`SameSite=Lax` — the server
expects a TLS-terminating reverse proxy in front of it. Without TLS
(e.g. a plain local test over `http://`), the browser won't send the
session cookie back.

## Testing without a real Authentik server

`internal/web/fakeoidc_test.go` provides a minimal, local fake identity
provider (discovery, JWKS, authorize, token — with genuinely signed ID
tokens), against which the tests in `server_test.go` play out the
complete login flow, including the "group missing → 403" case.
