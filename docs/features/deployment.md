# Deployment (Docker)

`dmarc-analyzer` läuft ausschließlich als Docker-Container, komplett über
Umgebungsvariablen konfiguriert (12-factor) — siehe
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md` für die
Begründung dieses Umstiegs.

## Schnellstart

```sh
docker run -d \
  --name dmarc-analyzer \
  -p 8080:8080 \
  -v dmarc-data:/data \
  --env-file .env \
  ghcr.io/pmoscode/dmarc-analyzer:latest
```

Eine vollständige Beispielkonfiguration inklusive aller Variablen liegt in
[`docker-compose.yml`](../../docker-compose.yml) im Repository-Wurzelverzeichnis.

Lokal bauen statt das veröffentlichte Image zu benutzen: `task docker:build`
(Taskfile.yml) bzw. direkt `docker build -t dmarc-analyzer .`.

## Umgebungsvariablen

### IMAP-Konto (Pflicht)

Genau ein Konto pro Container. Für mehrere Postfächer: mehrere Container
mit je eigenem `/data`-Volume.

| Variable | Pflicht | Vorgabe | Bedeutung |
| --- | --- | --- | --- |
| `DMARC_IMAP_HOST` | ja | — | IMAP-Host, z. B. `imap.example.com` |
| `DMARC_IMAP_PORT` | nein | `993` | IMAP-Port |
| `DMARC_IMAP_USER` | ja | — | Benutzername/E-Mail-Adresse |
| `DMARC_IMAP_PASSWORD` | ja | — | Passwort/App-Passwort — nie in ein Image bauen, nur zur Laufzeit setzen |
| `DMARC_IMAP_MAILBOX` | nein | `INBOX` | Postfach, aus dem Reports abgeholt werden |
| `DMARC_IMAP_TLS` | nein | `true` | IMAPS verwenden — `false` nur mit Bedacht (Klartext-IMAP) |

### Aufbewahrung und Hintergrund-Sync

| Variable | Pflicht | Vorgabe | Bedeutung |
| --- | --- | --- | --- |
| `DMARC_RETENTION_MONTHS` | nein | `24` | Reports älter als N Monate werden automatisch gelöscht. `0` = unbegrenzt |
| `DMARC_SYNC_INTERVAL_MINUTES` | nein | `60` | Abstand zwischen automatischen Hintergrund-Abgleichen. `0` = nur manuell über den Abgleich-Knopf |

Siehe [`retention.md`](retention.md) für Details zur Aufbewahrungsrichtlinie.

### Anmeldung (OIDC/Authentik, Pflicht)

Siehe [`auth.md`](auth.md) für den vollständigen Anmeldeablauf und das
Authentik-Setup.

| Variable | Pflicht | Bedeutung |
| --- | --- | --- |
| `DMARC_OIDC_ISSUER_URL` | ja | Authentik-Anwendungs-URL, z. B. `https://authentik.example.com/application/o/dmarc/` |
| `DMARC_OIDC_CLIENT_ID` | ja | Client-ID der Authentik-Anwendung |
| `DMARC_OIDC_CLIENT_SECRET` | ja | Client-Secret der Authentik-Anwendung |
| `DMARC_OIDC_REDIRECT_URL` | ja | Vollständige, öffentlich erreichbare Callback-URL, z. B. `https://dmarc.example.com/anmelden/callback` |
| `DMARC_OIDC_ADMIN_GROUP` | ja | Authentik-Gruppenname, der Zugriff gewährt (geprüft gegen den `groups`-Claim) |

### Betrieb

| Variable | Pflicht | Vorgabe | Bedeutung |
| --- | --- | --- | --- |
| `DMARC_DATA_DIR` | nein | `/data` | Verzeichnis für die SQLite-Datenbank — als Volume mounten |
| `DMARC_LISTEN_ADDR` | nein | `:8080` | Adresse, auf die der HTTP-Server bindet |
| `DMARC_DEV_MODE` | nein | `false` | Vorlagen/Statik von der Festplatte laden statt eingebettet (nur für lokale Entwicklung, Arbeitsverzeichnis muss die Repository-Wurzel sein) |

Fehlt eine Pflichtvariable oder ist ein Wert ungültig, bricht der Prozess
beim Start mit einer Fehlermeldung ab, die **alle** Probleme auf einmal
auflistet (`internal/infra/envconfig`) — nicht nur das erste, damit man
nicht mehrfach neu starten muss, um jede fehlende Variable einzeln zu
entdecken.

## Healthcheck

`GET /gesund` antwortet unauthentifiziert mit `200 OK`, solange der
HTTP-Server läuft (kein Datenbankzugriff — ein einzelner langsamer Query
soll den Container nicht fälschlich als "ungesund" markieren). Das
Docker-Image hat kein `curl`/`wget` (Distroless-Basis) — das Dockerfile
nutzt stattdessen `dmarc-analyzer healthcheck`, einen eigenen Unterbefehl
derselben Binärdatei, als `HEALTHCHECK`.

## Persistenz

Die SQLite-Datenbank liegt unter `$DMARC_DATA_DIR/dmarc.db`. Dieses
Verzeichnis muss als Volume gemountet werden — ohne Volume gehen alle
importierten Reports beim Entfernen des Containers verloren.

## Diagnose per `docker exec`

Neben dem Web-Server (`web`, auch ohne Argumente der Standard) bringt die
Binärdatei weitere Unterbefehle für Wartung/Debugging mit:

```sh
docker exec dmarc-analyzer /dmarc-analyzer stats --days 30
docker exec dmarc-analyzer /dmarc-analyzer sync

# Datei-Import: erst in den Container kopieren, dann importieren
# (oder direkt über die Web-Oberfläche unter /import per Drag & Drop).
docker cp report.xml dmarc-analyzer:/tmp/report.xml
docker exec dmarc-analyzer /dmarc-analyzer import /tmp/report.xml
```
