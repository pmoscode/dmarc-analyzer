# DMARC Analyzer

Ein eigenständiges Programm für macOS, Windows und Linux, das
DMARC-Aggregate-Reports (RUA) aus einem IMAP-Postfach abholt, lokal in
SQLite speichert und über eine im Programm eingebettete Web-Oberfläche
auswertet — kein separater Webserver, keine Installation, kein
Internetzugriff für die Oberfläche selbst.

> Details zu Architektur und Zielbild stehen in
> [`IMPLEMENTIERUNG.md`](IMPLEMENTIERUNG.md), der aktuelle Arbeitsplan in
> [`UMSETZUNGSPLAN.md`](UMSETZUNGSPLAN.md) und
> [`MIGRATIONSPLAN.md`](MIGRATIONSPLAN.md) (Umstieg von einer früheren
> Fyne-Desktop-Oberfläche auf die heutige Web-Oberfläche, siehe auch
> `docs/adr/`).

## Was das Programm tut

- Verbindet sich mit einem oder mehreren konfigurierten IMAP-Postfächern
  und holt **inkrementell nur neue** DMARC-Aggregate-Reports ab (`.eml`,
  `.xml`, `.xml.gz`, `.zip`, auch mit mehreren Reports je Anhang).
- Reports lassen sich zusätzlich per Drag & Drop oder Datei-Auswahl direkt
  in der Web-Oberfläche importieren (`/import`), ohne IMAP-Zugang.
- Speichert die ausgewerteten Daten lokal in SQLite — keine Cloud-Anbindung.
- Zeigt auf einer Übersicht Kennzahlen (Nachrichten gesamt, DMARC-Pass-Rate,
  DKIM-/SPF-Alignment-Rate, Anzahl unterschiedlicher Quellen), eine
  Zeitreihe des Nachrichtenvolumens, die Verteilung nach Disposition, die
  Top-Sendequellen und eine Sendequelle-×-Tag-Heatmap — alle vier Diagramme
  interaktiv (Chart.js: Tooltips, Zoom in der Zeitreihe, Klick auf einen
  Tag/eine Quelle/eine Zelle/ein Segment öffnet die passend gefilterten
  Berichte).
- Filtert, sortiert und gruppiert Berichte nach Zeitraum, Domain,
  Absender-Organisation und Quell-IP; Filter stehen in der URL
  (Lesezeichen und Zurück-Knopf funktionieren).
- Exportiert Berichte und Sendequellen als CSV (gesamter gefilterter
  Bestand, nicht nur die aktuell angezeigte Seite) sowie einzelne Diagramme
  als PNG oder CSV.
- Speichert Zugangsdaten ausschließlich im Schlüsselbund des
  Betriebssystems (Fallback unter Linux ohne Secret Service: verschlüsselte
  Datei mit eigener Master-Passphrase), niemals im Klartext in der
  Datenbank.

## Installation

Vorgefertigte Binärdateien für macOS (arm64/amd64), Windows (amd64) und
Linux (amd64/arm64) entstehen bei jedem Versions-Tag automatisch als
GitHub-Release-Anhang (siehe `.github/workflows/ci.yml`, Task
`release` in `Taskfile.yml`) — jeweils als `.zip` (macOS: `.app`-Bündel,
Windows: `.exe`) bzw. `.tar.gz` (Linux), zusammen mit einer
`checksums.txt`. Alternativ selbst bauen, siehe „Entwicklungs-Setup".

Auf macOS ist das Programm nicht notariell signiert — beim ersten
Doppelklick auf das `.app`-Bündel meldet Gatekeeper das Programm ggf. als
nicht verifiziert; über „Systemeinstellungen → Datenschutz & Sicherheit →
Trotzdem öffnen" (oder `xattr -d com.apple.quarantine dmarc-analyzer.app`)
lässt es sich dennoch starten.

## Start und Beenden

Ohne Argumente startet `dmarc-analyzer` (auch per Doppelklick) einen
lokalen HTTP-Server auf `127.0.0.1` mit einem zufälligen freien Port und
öffnet automatisch den Standardbrowser mit einem Einmal-Anmeldelink (60 s
gültig). Läuft bereits eine Instanz, holt sich ein zweiter Start nur einen
frischen Anmeldelink und öffnet ihn im selben Browser, statt einen
weiteren Server zu starten.

```sh
dmarc-analyzer                                   # Web-Oberfläche, Browser öffnet automatisch
dmarc-analyzer web --adresse 127.0.0.1:8080      # fester Port statt zufällig
dmarc-analyzer web --kein-browser                # Adresse nur auf der Konsole ausgeben
```

Beendet wird ausschließlich per **Strg+C oder SIGTERM** — es gibt bewusst
kein „Beenden"-Knopf in der Oberfläche und kein automatisches Ende nach
Leerlauf (der Prozess verhält sich wie ein gewöhnlicher Server-Dienst,
nicht wie eine Desktop-App mit eigenem Fenster; siehe
`MIGRATIONSPLAN.md` Entscheidung E-3). Wird nur der Browser-Tab
geschlossen, läuft der Prozess im Hintergrund weiter — ein erneuter Aufruf
von `dmarc-analyzer` oder ein Blick in die Konsolenausgabe des ursprünglich
gestarteten Prozesses liefert jederzeit einen neuen Anmeldelink.

Für Automatisierung (Cron/launchd/Taskplaner) stehen zusätzlich
Kommandozeilen-Unterbefehle bereit, die keinen Server starten:

```sh
dmarc-analyzer sync [--headless]                 # alle konfigurierten Konten synchronisieren
dmarc-analyzer import <pfad>...                  # Reports aus Dateien importieren
dmarc-analyzer stats [--days N] [--domain D]      # Kennzahlen auf der Konsole ausgeben
dmarc-analyzer account add|list|test|delete       # Konten verwalten
```

## Sicherheitsmodell

Ein Server auf `127.0.0.1` ist nicht automatisch privat (andere Benutzer
desselben Rechners, DNS-Rebinding, CSRF von anderen offenen Webseiten
gegen `127.0.0.1`) — daher:

- Bindung ausschließlich an `127.0.0.1`, `Host`-Header muss exakt
  `127.0.0.1:<port>` oder `localhost:<port>` sein.
- Der Browser bekommt nie ein dauerhaftes Geheimnis, sondern einen
  **Einmal-Anmeldelink** (60 s gültig), der gegen ein Sitzungs-Cookie
  (`HttpOnly`, `SameSite=Strict`) eingetauscht wird. Ohne gültige Sitzung
  liefert jede Anfrage 401.
- Jede zustandsändernde Anfrage läuft nur per POST mit CSRF-Token und
  Prüfung von `Origin`/`Sec-Fetch-Site`.
- Content-Security-Policy ohne `unsafe-inline` und ohne externe Quellen
  (`default-src 'self'`) — sämtliches JavaScript liegt in
  `internal/web/static/*.js`, nie inline im Template.
- Zugangsdaten werden nur per POST übertragen, nie an den Browser
  zurückgegeben und nie geloggt.

Details und Begründung: `MIGRATIONSPLAN.md` Abschnitt 5.

## Einrichtung des IMAP-Kontos

Ohne konfiguriertes Konto leitet die Web-Oberfläche automatisch auf eine
Ersteinrichtung (`/einrichtung`) weiter: Host, Port, Verschlüsselung,
Benutzername, App-Passwort (empfohlen bei aktivierter
Zwei-Faktor-Authentifizierung) und Postfachname/-ordner (per Verbindungstest
aus dem Postfach vorschlagbar). Weitere Konten lassen sich anschließend
unter „Einstellungen" ergänzen, testen oder entfernen. Alternativ per CLI:
`dmarc-analyzer account add`.

## Speicherorte

| Inhalt | Ort |
| --- | --- |
| Datenbank (`dmarc.db`) | `os.UserConfigDir()/dmarc-analyzer/` |
| Konfiguration, Instanz-Datei (`instance.json`) | `os.UserConfigDir()/dmarc-analyzer/` |
| Logs | macOS: `~/Library/Logs/dmarc-analyzer/dmarc-analyzer.log`, sonst wie Konfiguration |
| Zugangsdaten | OS-Schlüsselbund, Service `de.pmoscode.dmarc-analyzer` (Linux ohne Secret Service: verschlüsselte Datei im Konfigurationsverzeichnis) |

Die Log-Datei ergänzt die Konsolenausgabe (schreibt zusätzlich, nicht
statt dessen) — wichtig vor allem für den Windows-Release-Build, der ohne
Konsolenfenster läuft (`-H windowsgui`, siehe `Taskfile.yml`).

## Datenschutz

DMARC-Aggregate-Reports enthalten IP-Adressen von Mailversendern. Die
Verarbeitung erfolgt ausschließlich lokal, ohne Cloud-Übertragung. Eine
konfigurierbare Aufbewahrungsrichtlinie ist für Arbeitspaket 7 vorgesehen
(siehe `UMSETZUNGSPLAN.md`).

## Entwicklungs-Setup

Voraussetzungen: Go 1.27+, [Task](https://taskfile.dev/).

```sh
task setup     # Werkzeuge installieren, Abhängigkeiten laden
task check     # fmt + lint + test — vor jedem Commit, das auch die CI ausführt
task run       # Programm im Entwicklungsmodus starten (öffnet den Browser)
task build     # Binärdatei nach bin/ bauen (aktuelle Plattform)
task release   # Cross-Compile für alle Zielplattformen nach bin/release/
```

Alle verfügbaren Tasks: `task --list`. Details zu Architektur, Teststrategie
und Phasenplan stehen in [`IMPLEMENTIERUNG.md`](IMPLEMENTIERUNG.md).

## Lizenz

[MIT](LICENSE)
