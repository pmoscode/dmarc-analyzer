# ADR 0003: Rein Docker-nativ mit OIDC/Authentik-SSO statt Desktop-Distribution

- Status: angenommen
- Datum: 2026-09-17

## Kontext

ADR 0001 ersetzte die Fyne-Desktop-Oberfläche durch eine im Programm
eingebettete Web-Oberfläche, behielt aber implizit das Grundmodell der
Desktop-Ära bei: ein eigenständiges Programm, das direkt auf dem Rechner
des Nutzers läuft, IMAP-Zugangsdaten im OS-Schlüsselbund ablegt (mit
verschlüsseltem Datei-Fallback für Linux ohne Secret Service), Konten
interaktiv über eine "Ersteinrichtung" anlegt, sich per Einmal-Anmeldelink
selbst öffnet und nur auf `127.0.0.1` lauscht. Das war beim Umstieg auf die
Web-Oberfläche nicht mit hinterfragt worden — es stammte aus der Zeit vor
ADR 0001 und wurde einfach mitgenommen.

In der Praxis zeigte sich: die Web-Oberfläche eröffnet die Möglichkeit,
`dmarc-analyzer` stattdessen als gewöhnlichen Docker-Container zu
betreiben, zentral erreichbar statt auf einem einzelnen Arbeitsplatzrechner
installiert. Das macht mehrere Teile des bisherigen Sicherheits- und
Betriebsmodells gegenstandslos:

- Ein OS-Schlüsselbund ergibt in einem Container keinen Sinn — es gibt
  keinen "Nutzer-Desktop", dessen Schlüsselbund geöffnet werden könnte.
  IMAP-Zugangsdaten lassen sich stattdessen einfacher und
  container-üblich per Umgebungsvariable hereinreichen (12-factor).
- Plattformkonforme Pfade (`os.UserConfigDir()` je OS), Browser-Auto-Open
  und die dateibasierte Einzelinstanz-Erkennung sind reine
  Desktop-Konzepte — ein Container hat ein festes Datenverzeichnis
  (Volume) und läuft ohnehin nur einmal pro Container.
- Ein zentral erreichbarer Dienst statt "läuft nur auf 127.0.0.1, ein
  Nutzer pro Rechner" braucht echten Mehrbenutzer-Zugriffsschutz. Der
  bisherige Einmal-Anmeldelink (an der Konsole ausgegeben) setzt voraus,
  dass sich Konsole und Browser auf demselben Gerät befinden — das trifft
  auf einen Server nicht mehr zu. Es existiert bereits ein Authentik-IdP,
  gegen den sich anmelden lässt.

## Entscheidung

1. **IMAP-Zugangsdaten und alle Laufzeit-Einstellungen kommen
   ausschließlich aus Umgebungsvariablen** (`internal/infra/envconfig`,
   siehe `docs/features/deployment.md` für die vollständige Referenz).
   Genau ein Konto pro Container — kein Konto-CRUD mehr in der
   Oberfläche, kein Ersteinrichtungs-Assistent. Für mehrere Postfächer:
   mehrere Container mit je eigener Datenbank.
2. **Zugriffsschutz per OIDC gegen den bestehenden Authentik-IdP**
   (Authorization Code Flow mit PKCE, `internal/web/oidc.go`) statt eines
   Einmal-Anmeldelinks. Zugriff nur für Mitglieder einer konfigurierbaren
   Authentik-Gruppe (`DMARC_OIDC_ADMIN_GROUP`, geprüft gegen den
   `groups`-Claim des ID-Tokens). Sitzungen sind jetzt echte,
   parallel laufende Mehrbenutzer-Sitzungen (`internal/web/auth.go`),
   nicht mehr eine einzige globale Sitzung pro Prozess.
3. **Der Server bindet nicht mehr ausschließlich auf Loopback-Adressen** —
   der Host-Header wird stattdessen gegen den aus
   `DMARC_OIDC_REDIRECT_URL` abgeleiteten öffentlichen Hostnamen geprüft
   (`requireHost`, `internal/web/middleware.go`), der Zugriffsschutz kommt
   von OIDC, nicht mehr von "nur vom selben Rechner aus erreichbar".
4. **Auslieferung als Docker-Image** (`Dockerfile`, Multi-Stage,
   `CGO_ENABLED=0`, `gcr.io/distroless/static-debian12:nonroot` als
   Laufzeit-Basis) statt plattformspezifischer Binärdateien
   (`.app`-Bündel, `.exe`, `.tar.gz`) als GitHub-Release-Anhang. CI baut
   nur noch auf `ubuntu-latest` (siehe `.github/workflows/ci.yml`) und
   veröffentlicht bei einem Versions-Tag ein Image nach GHCR statt
   Release-Binärdateien für drei Plattformen zu bauen.

Entfernt: `internal/infra/keyring` (OS-Schlüsselbund + Datei-Fallback),
`internal/platform/paths` (plattformkonforme Pfade), `internal/web/browser.go`
(Browser-Auto-Open), `internal/web/instance.go` (Einzelinstanz-Erkennung
über `instance.json`), `internal/web/handlers_onboarding.go`
(Ersteinrichtungs-Assistent), `internal/web/handlers_unlock.go`
(Schlüsselbund-Sperrbildschirm), `packaging/darwin/` (`.app`-Bündel-Metadaten).

## Konsequenzen

**Vorteile:**

- Deutlich einfacheres Betriebsmodell: ein Container, eine ENV-Datei, ein
  Volume — kein "Wo liegt die Datenbank auf diesem Betriebssystem?", kein
  Schlüsselbund-Fallback-Fragenkatalog mehr.
- Echtes Mehrbenutzer-Rollenmodell statt "ein Nutzer pro installiertem
  Programm" — passt zu einem zentral betriebenen Dienst.
- CI/Release deutlich schlanker (ein Runner-Betriebssystem statt drei, ein
  Artefakttyp statt vier).

**Nachteile / bewusst in Kauf genommen:**

- Kein Offline-Betrieb ohne funktionierenden OIDC-Provider mehr — bewusst
  akzeptiert, weil der Anwendungsfall (zentral betriebenes Team-Werkzeug
  mit vorhandenem Authentik) das voraussetzt.
- Wer `dmarc-analyzer` weiterhin als reines Einzelplatz-Werkzeug ohne
  Authentik betreiben möchte, müsste einen eigenen minimalen OIDC-Provider
  aufsetzen oder auf eine ältere Version vor diesem ADR zurückgreifen —
  dieser Anwendungsfall wird nicht mehr aktiv unterstützt.
- Kein `docker exec`-loses CLI-Erlebnis mehr wie zuvor (`account add` etc.)
  — Konten kommen nur noch aus ENV, das reduziert Interaktivität zugunsten
  von Automatisierbarkeit.

## Alternativen

- **Forward-Auth/Outpost vor der App statt eigenem OIDC-Client:** Authentik
  als Reverse-Proxy-Middleware (z. B. Traefik-ForwardAuth) hätte die
  App selbst von jeder Auth-Logik befreit. Verworfen zugunsten eines
  App-nativen OIDC-Clients, damit die App unabhängig vom konkreten
  Reverse-Proxy bleibt und nicht zwingend hinter einem bestimmten
  Outpost-Aufbau betrieben werden muss.
- **Retention/Sync-Intervall weiterhin per Web-UI-Formular:** verworfen
  zugunsten von durchgängig reiner ENV-Konfiguration (12-factor) — sonst
  gäbe es zwei verschiedene Konfigurationswege nebeneinander.
