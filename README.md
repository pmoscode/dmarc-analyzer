# DMARC Analyzer

Ein als Docker-Container betriebenes Programm, das DMARC-Aggregate-Reports
(RUA) aus einem IMAP-Postfach abholt, in SQLite speichert und über eine
eingebettete Web-Oberfläche auswertet. Zugriffsschutz per OIDC gegen einen
bestehenden Authentik-Identity-Provider, komplett über Umgebungsvariablen
konfiguriert.

> Architektur: [`docs/architecture.md`](docs/architecture.md). Feature-Doku
> pro Domäne: [`docs/features/`](docs/features/). Architekturentscheidungen
> samt Begründung: [`docs/adr/`](docs/adr/). `docs/archive/` enthält die
> Planungsdokumente der früheren Desktop-Ära — historischer Kontext, kein
> aktueller Stand mehr.

## Was das Programm tut

- Verbindet sich mit einem konfigurierten IMAP-Postfach und holt
  **inkrementell nur neue** DMARC-Aggregate-Reports ab (`.xml`, `.xml.gz`,
  `.zip`, auch mit mehreren Reports je Anhang) — siehe
  [`docs/features/imap-sync.md`](docs/features/imap-sync.md).
- Reports lassen sich zusätzlich per Drag & Drop oder Datei-Auswahl direkt
  in der Web-Oberfläche importieren (`/import`), ohne IMAP-Zugang.
- Zeigt auf einer Übersicht Kennzahlen (Nachrichten gesamt, DMARC-Pass-Rate,
  DKIM-/SPF-Alignment-Rate, Anzahl unterschiedlicher Quellen) und vier
  interaktive Diagramme (Chart.js: Zeitreihe, Disposition, Top-Sendequellen,
  Sendequelle-×-Tag-Heatmap) mit Drill-down in die Berichtstabelle — siehe
  [`docs/features/dashboard.md`](docs/features/dashboard.md).
- Filtert, sortiert und gruppiert Berichte und Sendequellen nach Zeitraum,
  Domain, Absender-Organisation und Quell-IP; Filter stehen in der URL
  (Lesezeichen und Zurück-Knopf funktionieren) — siehe
  [`docs/features/reports.md`](docs/features/reports.md) und
  [`docs/features/sources.md`](docs/features/sources.md).
- Exportiert Berichte und Sendequellen als CSV (gesamter gefilterter
  Bestand) sowie einzelne Diagramme als PNG oder CSV.
- Löscht Reports automatisch nach einer konfigurierbaren
  Aufbewahrungsdauer — siehe
  [`docs/features/retention.md`](docs/features/retention.md).
- Zugriffsschutz per OIDC/Authentik, nur für Mitglieder einer
  konfigurierbaren Gruppe — siehe
  [`docs/features/auth.md`](docs/features/auth.md).

## Schnellstart

```sh
docker run -d \
  --name dmarc-analyzer \
  -p 8080:8080 \
  -v dmarc-data:/data \
  --env-file .env \
  ghcr.io/pmoscode/dmarc-analyzer:latest
```

Vollständige Umgebungsvariablen-Referenz und ein Beispiel mit allen
Werten: [`docs/features/deployment.md`](docs/features/deployment.md) bzw.
[`docker-compose.yml`](docker-compose.yml).

Voraussetzung ist ein bestehender Authentik-Identity-Provider — Setup-
Anleitung in [`docs/features/auth.md`](docs/features/auth.md).

## Entwicklungs-Setup

Voraussetzungen: Go 1.27+, [Task](https://taskfile.dev/), Docker (für
`task docker:build`/`docker:run`).

```sh
task setup         # Werkzeuge installieren, Abhängigkeiten laden
task check         # fmt + lint + test — vor jedem Commit, das auch die CI ausführt
task run           # Programm im Entwicklungsmodus starten (Web-Oberfläche)
task build         # Binärdatei nach bin/ bauen
task docker:build  # Docker-Image lokal bauen
```

Alle verfügbaren Tasks: `task --list`.

## Lizenz

[MIT](LICENSE)
