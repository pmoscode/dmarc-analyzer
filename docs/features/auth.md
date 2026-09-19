# Anmeldung (OIDC/Authentik)

Der Zugriff auf die Web-Oberfläche ist über OpenID Connect gegen einen
bestehenden [Authentik](https://goauthentik.io/)-Identity-Provider
abgesichert — nur Mitglieder einer konfigurierbaren Authentik-Gruppe
dürfen die Anwendung sehen. Begründung für dieses Modell (statt eines
Einmal-Anmeldelinks wie in der früheren Desktop-Version):
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`.

## Ablauf (Authorization Code Flow + PKCE)

1. `GET /anmelden` — erzeugt einen kurzlebigen Anmeldevorgang (`state`,
   `nonce`, PKCE-Verifier), legt ihn serverseitig ab und leitet zu
   Authentiks Authorize-Endpoint weiter (per OIDC-Discovery aus
   `DMARC_OIDC_ISSUER_URL` ermittelt).
2. Authentik authentifiziert den Nutzer und leitet zurück zu
   `DMARC_OIDC_REDIRECT_URL` (muss exakt `<öffentliche-URL>/anmelden/callback`
   sein und in Authentik als Redirect-URI eingetragen werden).
3. `GET /anmelden/callback` — tauscht den Code gegen Tokens, validiert das
   ID-Token (Issuer, Audience, Signatur, Ablauf, Nonce) und prüft, ob der
   `groups`-Claim `DMARC_OIDC_ADMIN_GROUP` enthält. Fehlt die
   Gruppenmitgliedschaft, gibt es `403 Forbidden` statt einer Sitzung —
   kein automatischer erneuter Redirect zu Authentik (das würde bei
   dauerhaft fehlender Berechtigung zu einer Schleife führen).
4. Bei Erfolg wird eine Sitzung angelegt (Cookie `dmarc_session`, 12 h
   gültig) und auf die Übersicht weitergeleitet.
5. `POST /abmelden` beendet die Sitzung und leitet — falls Authentiks
   Discovery-Dokument einen `end_session_endpoint` bekanntgibt — dorthin
   weiter (RP-initiated Logout), sonst auf `/anmelden`. Die Content-Security-Policy
   (`internal/web/middleware.go`) erlaubt dafür in `form-action` neben `'self'`
   zusätzlich die Origin von `DMARC_OIDC_ISSUER_URL` — ohne das würde der
   Browser den Redirect zu einer anderen Origin blocken, die lokale Sitzung
   wäre zwar beendet, Authentik bekäme den Logout aber nie mitgeteilt.

Mehrere Admins können gleichzeitig von verschiedenen Browsern angemeldet
sein — jede Sitzung ist unabhängig (`internal/web/auth.go`).

## Authentik-Setup

1. Neue OAuth2/OpenID-Provider-Anwendung in Authentik anlegen.
2. Redirect-URI exakt auf `<öffentliche-URL>/anmelden/callback` setzen.
3. Scopes: mindestens `openid`, `profile`, `email`, **`groups`** (der
   `groups`-Claim im ID-Token ist Voraussetzung für die
   Zugriffsprüfung — ohne diesen Scope bleibt der Claim leer und
   niemand kann sich anmelden).
4. Eine Gruppe anlegen (oder eine bestehende verwenden), deren Namen als
   `DMARC_OIDC_ADMIN_GROUP` eingetragen wird, und die berechtigten Nutzer
   dieser Gruppe zuweisen.
5. Client-ID/Client-Secret aus Authentik in `DMARC_OIDC_CLIENT_ID`/
   `DMARC_OIDC_CLIENT_SECRET` übernehmen.

## Netzwerk-Zugriffsschutz

Der Server prüft zusätzlich den `Host`-Header jeder Anfrage gegen den aus
`DMARC_OIDC_REDIRECT_URL` abgeleiteten öffentlichen Hostnamen (`requireHost`, `internal/web/middleware.go`) — ein
Reverse-Proxy muss den
externen Host-Header unverändert durchreichen, sonst antwortet der Server
mit `421 Misdirected Request`.

Cookies sind `Secure`+`HttpOnly`+`SameSite=Lax` gesetzt — der Server
erwartet einen TLS-terminierenden Reverse-Proxy davor. Ohne TLS (z. B. reiner lokaler Test über `http://`) wird der
Browser das
Sitzungs-Cookie nicht zurücksenden.

## Testen ohne echten Authentik-Server

`internal/web/fakeoidc_test.go` stellt einen minimalen, lokalen
Fake-Identity-Provider bereit (Discovery, JWKS, Authorize, Token — mit
echt signierten ID-Tokens), gegen den die Tests in `server_test.go` den
vollständigen Anmeldeablauf durchspielen, inklusive des Falls "Gruppe
fehlt → 403".
