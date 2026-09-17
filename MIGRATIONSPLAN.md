# Migrationsplan — Fyne-Oberfläche → eingebettete Web-Oberfläche

> Stand: 2026-09-17. Ergänzt `UMSETZUNGSPLAN.md` und ist dort **vor AP 7**
> einzuordnen (Packaging und Feinschliff hängen vom Ergebnis ab).
> Fortschritt: M0 und M1 umgesetzt (siehe Abschnitt 10), M2 offen.

## 1. Anlass und Ziel

Die Fyne-Oberfläche wirkt trotz Überarbeitung schwerfällig („klobig"). Ziel ist
eine **Web-Oberfläche, die vollständig in der Binärdatei steckt**: Beim Start
läuft ein lokaler HTTP-Server, der Standardbrowser öffnet automatisch die
Hauptseite. Keine Installation eines Webservers, keine externen Dateien, kein
Internetzugriff für die Oberfläche.

**Vorgabe, die sich dadurch ändert:** `FEATURES.md` nennt unter „Non features"
ausdrücklich Fyne als UI-Framework und „Fyne framework best practices";
`IMPLEMENTIERUNG.md` Abschnitt 1.2 hat das als verbindlich behandelt. Diese
Migration ersetzt die Vorgabe bewusst — `FEATURES.md` wird in M5 entsprechend
angepasst (siehe Abschnitt 12).

### Was die Migration zusätzlich bringt

| Gewinn                                            | Warum                                                                                                                                                                                                                                                                                                                                                  |
|---------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Kein CGO mehr**                                 | Fyne ist die einzige CGO-Abhängigkeit. Geprüft am 2026-09-17: alle Pakete außer `internal/ui` bauen mit `CGO_ENABLED=0` für `linux/amd64`, `windows/amd64` und `darwin/arm64`. Releases entstehen dann per Cross-Compile auf einem Rechner.                                                                                                            |
| **Interaktive Diagramme statt statischer Bilder** | Heute sind Diagramme vom Server gerenderte PNGs: kein Tooltip, kein Ein-/Ausblenden, kein Zoom, kein Klick. Künftig zeichnet Chart.js im Browser — mit Tooltips, Legende zum Umschalten, Zoom in der Zeitreihe und **Drill-down**: ein Klick auf einen Tag, eine Sendequelle, eine Heatmap-Zelle oder ein Donut-Segment öffnet die passenden Berichte. |
| **Diagramme passen sich hell/dunkel an**          | Heute haben die PNGs einen fest weißen Hintergrund (`flattenOnWhite`) — im dunklen Modus helle Kästen. Chart.js liest die Farben aus CSS-Variablen und zeichnet beim Wechsel des Farbschemas neu.                                                                                                                                                      |
| **Echte Tooltips**                                | Fyne v2.8 hat keine; heute ersetzt ein „?"-Knopf sie. Im Browser: Hover-Tooltips auf Diagrammen und Begriffen.                                                                                                                                                                                                                                         |
| **Weniger Umgehungslösungen**                     | `internal/infra/charts` enthält mehrere go-chart-Workarounds (Textumbruch, Ein-Wert-Donut, transparente Ränder), `internal/ui` mehrere Fyne-Fallen (siehe `AGENTS.md`). Beides entfällt.                                                                                                                                                               |
| **Lesezeichen und Zurück-Knopf**                  | Filter stehen in der URL (`?zeitraum=30&domain=…`).                                                                                                                                                                                                                                                                                                    |

## 2. Was bleibt, was sich ändert

Die Clean Architecture trägt die Migration: **nur die Präsentationsschicht wird
ausgetauscht.** Domäne, Use Cases und Adapter bleiben, mit vier kleinen,
gezielten Erweiterungen (Abschnitt 9).

```
                 heute                                   nach der Migration
cmd/dmarc-analyzer ──▶ internal/ui (Fyne)      cmd/dmarc-analyzer ──▶ internal/web (net/http)
                          │                                                 │
                          ▼                                                 ▼
                   internal/app/*        (unverändert, + Sync-Fortschritt, Import aus Bytes)
                          │
                          ▼
            internal/domain/*  ◀──  internal/infra/*   (unverändert, + sperrbarer Schlüsselspeicher)
```

| Bereich                                                                                       | Umgang                                                  |
|-----------------------------------------------------------------------------------------------|---------------------------------------------------------|
| `internal/domain/*`, `internal/app/*`, `internal/infra/*`                                     | bleiben; Erweiterungen siehe Abschnitt 9                |
| CLI-Unterbefehle (`sync`, `import`, `stats`, `account`)                                       | bleiben unverändert                                     |
| `internal/ui/i18n` (reine Konstanten)                                                         | wird nach `internal/web/i18n` verschoben, Inhalt bleibt |
| `internal/ui/glossary/terms.go` (reine Daten)                                                 | wird nach `internal/web/glossary` verschoben            |
| Farbwerte aus `internal/ui/theme.go`                                                          | werden zu CSS-Variablen (hell/dunkel), Werte bleiben    |
| restliches `internal/ui/*` (~4.900 Zeilen inkl. Tests)                                        | wird in M5 gelöscht                                     |
| `internal/infra/charts` (go-chart), Port `analysis.ChartRenderer`, `exportdata.WriteChartPNG` | entfallen — Diagramme entstehen im Browser (E-2)        |

## 3. Zielbild im Betrieb

| Situation                                                      | Verhalten                                                                                                                                                  |
|----------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `dmarc-analyzer` ohne Argumente (auch Doppelklick)             | Server auf `127.0.0.1` mit zufälligem freiem Port starten, Browser mit Einmal-Anmeldelink öffnen, Adresse zusätzlich auf der Konsole ausgeben              |
| Programm läuft bereits                                         | Zweiter Start erkennt die laufende Instanz, lässt sie einen neuen Einmal-Link erzeugen, öffnet ihn im Browser und beendet sich sofort                      |
| Keine Konten vorhanden                                         | `/` leitet auf die Ersteinrichtung um                                                                                                                      |
| `dmarc-analyzer serve --adresse 127.0.0.1:8080 --kein-browser` | fester Port, kein Browserstart (Entwicklung, SSH-Sitzung)                                                                                                  |
| Beenden                                                        | Strg+C / SIGTERM (siehe E-3) — kein Auto-Ende, kein Knopf in der Oberfläche |
| Kein Betriebssystem-Schlüsselbund (Linux ohne Secret Service)  | Oberfläche zeigt zuerst eine Entsperr-Seite für die Master-Passphrase — heute fragt das Programm auf der Konsole, die es beim Doppelklick-Start nicht gibt |
| CLI-Unterbefehle                                               | unverändert, kein Server                                                                                                                                   |

**Browser öffnen** ohne zusätzliche Abhängigkeit per `os/exec`: `open` (macOS),
`xdg-open` (Linux), `rundll32 url.dll,FileProtocolHandler` (Windows). Schlägt
das fehl, bleibt die Adresse auf der Konsole — kein Abbruch.

## 4. Entscheidungen

Vorbelegt ist jeweils die Empfehlung. Bitte bestätigen oder ändern, bevor M0
beginnt.

| Nr. | Frage                                  | Empfehlung                                                                                                                                                                                                                                                                                                    | Alternative                                                                                   | Begründung                                                                                                                                                                                                                                                                                                                                     |
|-----|----------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| E-1 | Frontend-Technik                       | **`html/template` (Standardbibliothek) + htmx** für Seiten, Tabellen und Formulare; eigenes JavaScript nur für die Diagramme (Abschnitt 6a); alles eingebettet, **ohne Bundler**                                                                                                                              | SPA (Svelte/Vue/React) mit Vite-Build                                                         | Kein Node-Werkzeug in Entwicklung und CI, `go build` bleibt der einzige Build-Schritt. Tabellen und Formulare deckt serverseitiges Rendern gut ab; die Interaktivität, die wirklich zählt, liefern die Diagramme. Ein SPA verdoppelt Ökosysteme und Tests.                                                                                     |
| E-2 | Diagramme                              | **Festgelegt: Chart.js im Browser** (Vorgabe), ergänzt um das Plugin `chartjs-chart-matrix` für die Heatmap und `chartjs-plugin-zoom` für die Zeitreihe. Der Server liefert nur Daten als JSON. go-chart, `internal/infra/charts` und `analysis.ChartRenderer` entfallen.                                     | Apache ECharts (Heatmap, Zoom und Bild-Export eingebaut, aber ≈1 MB und eigenes Theme-System) | Chart.js ist klein, verbreitet und bringt Tooltips, umschaltbare Legenden und Klick-Ereignisse mit. Die Heatmap ist kein Kerntyp von Chart.js — dafür das Matrix-Plugin. **Rückfallebene:** Zeigt der Durchstich (M0), dass die Plugin-Kombination nicht trägt, wird auf ECharts gewechselt; nur `charts.js` und die JSON-Form sind betroffen. |
| E-3 | Lebenszyklus                           | **Nur Strg+C / SIGTERM** (`context.Context` mit `signal.NotifyContext`, sauberes Herunterfahren über `http.Server.Shutdown`) — kein Auto-Ende, kein „Beenden"-Knopf, kein Tray-Icon | „Beenden"-Knopf + Auto-Ende nach Leerlauf · Tray-Icon | Der Prozess soll sich wie ein gewöhnlicher Server-Dienst verhalten, nicht wie eine Desktop-App mit eigenem Fenster — geplanter Einsatz auch als Docker-Image (siehe unten), wo der Lebenszyklus vom Container-Orchestrator kommt (`docker stop` sendet SIGTERM) und "offene Browser-Tabs zählen" keine sinnvolle Grundlage ist. Vereinfacht nebenbei den Server erheblich: keine SSE-Verbindungszählung, kein Timer, kein `--dauerhaft`-Flag nötig — das ist ab jetzt einfach der einzige Modus. AP 7 (Hintergrund-Sync) profitiert direkt: der Prozess läuft ohnehin, bis er gestoppt wird. |
| E-4 | Port                                   | **zufällig** (`127.0.0.1:0`), fest nur per Flag                                                                                                                                                                                                                                                               | fester Standardport                                                                           | Keine Konflikte; Lesezeichen funktionieren über die Einzelinstanz-Erkennung trotzdem, weil jeder Start die richtige Adresse öffnet.                                                                                                                                                                                                            |
| E-5 | Übergang                               | **Web-Oberfläche parallel in `internal/web` aufbauen**, Fyne bleibt Standard bis zur Funktionsgleichheit (M3), dann umschalten und Fyne in M5 löschen                                                                                                                                                         | sofort umschalten                                                                             | Das Programm ist jederzeit benutzbar. Vor 1.0 gibt es keine Nutzer, die ein Parallelangebot bräuchten — Fyne bleibt deshalb nicht dauerhaft als Option.                                                                                                                                                                                        |
| E-6 | Neue Möglichkeiten in dieser Migration | **Datei-Import per Upload/Drag & Drop mit aufnehmen** (Vorschlag 11.1 war bisher nur per CLI möglich); Filter in der URL; sortierbare Tabellenköpfe                                                                                                                                                           | nur 1:1-Übertrag                                                                              | Der Import-Dialog ersetzt ohnehin den Fyne-Dateidialog; der Mehraufwand ist klein.                                                                                                                                                                                                                                                             |
| E-7 | Bezug von htmx, Chart.js und Plugins   | **Minifizierte UMD-Dateien im Repository ablegen** (`internal/web/static/vendor/`), Versionen und Lizenzen (alle MIT) in `docs/DEPENDENCIES.md` pinnen; die genauen Versionen und die Kompatibilität der Plugins zur Chart.js-Hauptversion bei Einbindung recherchieren                                       | CDN                                                                                           | Funktioniert offline, passt zur Content-Security-Policy `default-src 'self'`.                                                                                                                                                                                                                                                                  |
| E-8 | Browser-Tests                          | **Ein schlanker Rauchtest mit `chromedp`** (Go, kein Node): Übersicht laden, prüfen, dass alle vier Diagramme gezeichnet sind und die Konsole keine Fehler meldet, einen Drill-down-Klick auslösen. Läuft lokal und auf dem Linux-Runner der CI; Handler-Tests mit `httptest` decken Logik und Sicherheit ab. | keine Browser-Tests · Playwright (braucht Node)                                               | Mit Chart.js steckt echte Logik im JavaScript (Datenzuordnung, Farben, Klickziele), die `httptest` nicht sieht. Ob Chrome auf dem CI-Runner vorhanden ist oder installiert werden muss, wird in M2 geprüft.                                                                                                                                    |

## 5. Sicherheit des lokalen Servers

Ein Server auf `127.0.0.1` ist **nicht automatisch privat**: andere Benutzer
desselben Rechners können sich verbinden, und jede geöffnete Webseite kann
Anfragen an `127.0.0.1` schicken (CSRF, DNS-Rebinding). Die Oberfläche
verwaltet Zugangsdaten — daher:

| Maßnahme                     | Umsetzung                                                                                                                                                                                                                                                                                                                                   |
|------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Nur Loopback                 | Bindung ausschließlich an `127.0.0.1`; andere Adressen werden abgelehnt. **Spannung mit dem geplanten Docker-Einsatz (E-3):** Ein Prozess, der in einem Container nur an `127.0.0.1` bindet, ist über `docker run -p …` von außen **nicht** erreichbar — Docker leitet an die Container-Schnittstelle weiter, nicht an deren Loopback. Für den Docker-Fall wird also eine andere Bindung (`0.0.0.0` im Container, oder `--network=host`) und damit ein anderes Bedrohungsmodell nötig, sobald das Docker-Image ansteht: Der Server wäre dann potenziell von anderen Rechnern im Netz erreichbar, nicht mehr nur von Prozessen auf derselben Maschine. Diese Migration hier baut nur das lokale (Loopback-)Modell; **eine spätere Docker-Verpackung braucht einen eigenen Sicherheits-Nachtrag** (u. a.: reicht das Einmal-Link-Verfahren noch, oder braucht es dann ein echtes Login mit Passwort und TLS?) — bewusst nicht Teil dieses Plans, siehe offene Frage 4. |
| Instanz-Geheimnis            | Beim Start 32 Zufallsbytes (`crypto/rand`), abgelegt in `instance.json` im Konfigurationsverzeichnis mit Rechten `0600`, zusammen mit Port und PID                                                                                                                                                                                          |
| Einmal-Anmeldelink           | Der Browser bekommt nicht das Geheimnis, sondern einen **einmal gültigen Code** (60 s gültig). `/anmelden?code=…` tauscht ihn gegen ein Sitzungs-Cookie (`HttpOnly`, `SameSite=Strict`) und leitet weiter — im Verlauf bleibt nur ein verbrauchter Code. Ein zweiter Programmstart holt sich mit dem Instanz-Geheimnis einen frischen Code. |
| Jede Anfrage authentifiziert | Ohne gültiges Sitzungs-Cookie: 401 und Hinweisseite „Bitte über das Programm öffnen"                                                                                                                                                                                                                                                        |
| DNS-Rebinding                | `Host`-Header muss exakt `127.0.0.1:<port>` oder `localhost:<port>` sein                                                                                                                                                                                                                                                                    |
| CSRF                         | Zustandsändernde Anfragen nur per POST, mit CSRF-Token (Formularfeld bzw. `hx-headers`) **und** Prüfung von `Origin` bzw. `Sec-Fetch-Site`                                                                                                                                                                                                  |
| Content-Security-Policy      | `default-src 'self'; frame-ancestors 'none'; form-action 'self'` — kein Inline-JavaScript, keine externen Quellen. Ob Chart.js und die Plugins ohne `'unsafe-inline'` bei `style-src` auskommen, wird im Durchstich (M0) geprüft, nicht vorausgesetzt.                                                                                      |
| JSON-Endpunkte               | Gleiche Sitzungs- und `Host`-Prüfung wie Seiten; nur GET, keine Zustandsänderung; `Content-Type: application/json` und `X-Content-Type-Options: nosniff`                                                                                                                                                                                    |
| Zugangsdaten                 | Passwörter nur per POST, nie zurück an den Browser, nie geloggt; `account.Secret` wie bisher direkt nach Gebrauch mit `Zero()` überschreiben                                                                                                                                                                                                |
| Uploads                      | Größenbegrenzung per `http.MaxBytesReader` (Vorschlag: 50 MB je Datei); das 100-MB-Entpacklimit aus AP 1 greift zusätzlich                                                                                                                                                                                                                  |
| Aufräumen                    | `instance.json` beim Beenden löschen; ein verwaister Eintrag (Prozess tot) wird beim nächsten Start übernommen                                                                                                                                                                                                                              |

## 6. Paketstruktur

```
internal/web/
├── server.go          # http.Server, Start/Stop über signal.NotifyContext, Einzelinstanz
├── browser.go         # Browser öffnen je Betriebssystem
├── auth.go            # Instanz-Geheimnis, Einmal-Codes, Sitzung, CSRF
├── middleware.go      # Host-Prüfung, Sicherheits-Header, Recover, Logging
├── routes.go          # Routing (net/http ServeMux mit Methoden-Mustern)
├── handlers_*.go      # je Ansicht: dashboard, reports, sources, accounts, onboarding, sync, import, export
├── api_charts.go      # JSON-Daten für die Diagramme (Abschnitt 6a)
├── views.go           # Template-Laden (eingebettet; mit --entwicklung von der Festplatte)
├── i18n/              # verschoben aus internal/ui/i18n
├── glossary/          # verschoben aus internal/ui/glossary (nur Daten)
├── templates/         # layout.html, partials/*.html, pages/*.html
└── static/
    ├── app.css        # Theme-Variablen hell/dunkel
    ├── app.js         # Tooltips für Begriffe, SSE (Sync-Fortschritt), Dialoge
    ├── charts.js      # Chart.js-Konfiguration, Farben aus CSS-Variablen, Drill-down
    └── vendor/        # htmx, chart.js, chartjs-chart-matrix, chartjs-plugin-zoom (jeweils .min.js + LICENSE)
```

`templates/` und `static/` werden per `//go:embed` eingebettet. Mit
`--entwicklung` liest der Server sie stattdessen von der Festplatte — Änderungen
an HTML/CSS/JS sind dann ohne Neubau sichtbar.

## 6a. Diagramme mit Chart.js

**Aufgabenteilung:** Der Server liefert fertig aufbereitete Daten; der Browser
zeichnet. Aggregation, Filter und Anreicherung (PTR, Dienstname) bleiben in Go
und damit testbar — `charts.js` ordnet nur zu und reagiert auf Klicks.

| Diagramm                                        | Chart.js-Typ                                                     | Interaktion                                                                                                             | Drill-down beim Klick                                                          |
|-------------------------------------------------|------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|
| Nachrichtenvolumen pro Tag, gestapelt Pass/Fail | `bar` (gestapelt)                                                | Tooltip mit Anzahl und Anteil je Tag; Legende blendet Pass/Fail ein und aus; Zoom und Verschieben bei langen Zeiträumen | Berichte dieses Tages                                                          |
| Top-Sendequellen                                | `bar` (horizontal, damit lange Dienst-/Hostnamen lesbar bleiben) | Tooltip mit IP, Hostname, Dienst, Volumen, Pass-Rate                                                                    | Sendequellen-Ansicht bzw. Berichte, gefiltert auf diese IP                     |
| Verteilung nach Disposition                     | `doughnut`                                                       | Tooltip mit Anzahl und Anteil; Segmente per Legende ausblendbar                                                         | Berichte mit dieser Disposition (`report.Query.Disposition` existiert bereits) |
| Sendequelle × Tag                               | `matrix` (Plugin)                                                | Tooltip mit Quelle, Tag, Pass-Rate **und Nachrichtenzahl**; leere Tage deutlich als „keine Daten"                       | Berichte dieser Quelle an diesem Tag                                           |

**JSON-Endpunkte** (gleiche Filterparameter wie die Seite, z. B.
`?zeitraum=30&domain=example.com`):

| Pfad                         | Inhalt                                                   |
|------------------------------|----------------------------------------------------------|
| `/api/diagramme/verlauf`     | Tage mit `pass`, `fail`                                  |
| `/api/diagramme/quellen`     | Quellen mit `ip`, `label`, `total`, `passRate`           |
| `/api/diagramme/disposition` | Anteile je Disposition                                   |
| `/api/diagramme/heatmap`     | Quellen, Tage, Zellen mit `passRate`, `total`, `hasData` |

Jede Antwort enthält zu jedem Punkt bereits die **Ziel-URL für den Drill-down**
(vom Server gebaut) — `charts.js` muss keine Filterlogik kennen.

**Gestaltung nach der `dataviz`-Skill:**

- Farben kommen aus denselben CSS-Variablen wie die Oberfläche (validierte
  Referenzpalette: Statusfarben für Pass/Fail/Disposition, Primärblau).
  `charts.js` liest sie per `getComputedStyle` und zeichnet bei
  `prefers-color-scheme`-Wechsel neu.
- Pass-Rate-Farbskala für Top-Quellen und Heatmap als Verlauf zwischen den
  Statusfarben „kritisch" und „gut"; „keine Daten" neutral grau.
- Schlanke Balken mit abgerundeten Enden, dezente Gitterlinien, Legende ab zwei
  Reihen, Beschriftungen in Textfarbe statt Reihenfarbe.
- **Barrierefreiheit:** Ein `<canvas>` ist für Screenreader leer. Jedes
  Diagramm bekommt eine Kurzbeschreibung (`aria-label`) und eine umschaltbare **Tabellenansicht** mit denselben Daten;
  Drill-down-Ziele sind dort normale
  Links und damit per Tastatur erreichbar.
- Leere Zeiträume zeigen den bestehenden Leerzustand statt eines leeren
  Diagramms.

**Export:** PNG über `chart.toBase64Image()` im Browser, CSV der
Diagrammdaten aus der Tabellenansicht. Serverseitiger Bild-Export entfällt.

**Domänen-Ergänzung:** `analysis.HeatmapCell` bekommt zusätzlich die
Nachrichtenzahl (`Total`), damit der Tooltip sie zeigen kann — die
SQL-Aggregation berechnet sie bereits, sie wird nur bisher verworfen.

## 7. Routen

| Methode  | Pfad                                                                                          | Zweck                                                                                                                                 |
|----------|-----------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| GET      | `/anmelden`                                                                                   | Einmal-Code gegen Sitzung tauschen                                                                                                    |
| POST     | `/intern/code`                                                                                | Neuen Einmal-Code ausstellen (nur mit Instanz-Geheimnis, für den zweiten Programmstart)                                               |
| GET      | `/`                                                                                           | Übersicht (bzw. Weiterleitung zur Ersteinrichtung)                                                                                    |
| GET      | `/api/diagramme/{verlauf,quellen,disposition,heatmap}`                                        | Diagrammdaten als JSON (Abschnitt 6a)                                                                                                 |
| GET      | `/berichte`                                                                                   | Berichtstabelle; Filter (auch Quell-IP, Disposition, einzelner Tag — für den Drill-down), Sortierung, Gruppierung als Query-Parameter |
| GET      | `/berichte/seite`                                                                             | nächste Seite (htmx, Keyset-Cursor)                                                                                                   |
| GET      | `/berichte/{id}`                                                                              | Detailansicht (als Dialog per htmx und als eigene Seite)                                                                              |
| GET      | `/quellen`, `/quellen/seite`                                                                  | Sendequellen                                                                                                                          |
| GET      | `/glossar`                                                                                    | Glossar                                                                                                                               |
| GET      | `/einstellungen`                                                                              | Kontenliste                                                                                                                           |
| POST     | `/konten` · `/konten/test` · `/konten/ordner` · `/konten/{id}/test` · `/konten/{id}/loeschen` | Anlegen, Verbindung testen, Ordner auflisten, Test gespeichertes Konto, Löschen                                                       |
| GET/POST | `/einrichtung`                                                                                | Ersteinrichtung in drei Schritten                                                                                                     |
| POST     | `/abgleich` · `/abgleich/abbrechen`                                                           | Sync starten/abbrechen                                                                                                                |
| GET      | `/ereignisse`                                                                                 | Server-Sent Events: Sync-Fortschritt, Ergebnis                                                      |
| POST     | `/import`                                                                                     | Datei-Upload                                                                                                                          |
| GET      | `/export/berichte.csv` · `/export/quellen.csv`                                                | Downloads (Diagramm-Export läuft im Browser)                                                                                          |
| GET/POST | `/entsperren`                                                                                 | Master-Passphrase für den Datei-Schlüsselspeicher                                                                                     |
| POST     | `/beenden`                                                                                    | Programm beenden                                                                                                                      |
| GET      | `/static/…`                                                                                   | eingebettete Dateien mit Cache-Headern                                                                                                |

## 8. Funktionsgleichheit — Übertrag der bestehenden Oberfläche

| Heute (Fyne)                                           | Künftig (Web)                                                               | Anmerkung                                                                                                     |
|--------------------------------------------------------|-----------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| Hauptfenster, Seitennavigation mit Symbolen, Kopfzeile | `layout.html` mit Navigationsleiste, Kopfzeile                              | Symbole als eingebettete SVG                                                                                  |
| Theme hell/dunkel (`appTheme`)                         | CSS-Variablen, `prefers-color-scheme`                                       | dieselben Farbwerte (dataviz-Referenzpalette)                                                                 |
| Filterleiste (Zeitraum, Domain)                        | GET-Formular, Werte in der URL                                              | wirkt weiter auf Übersicht, Berichte, Quellen                                                                 |
| Übersicht: 5 Kennzahlen-Kacheln, Trend                 | Kacheln als Karten, Trendpfeil als SVG-Symbol                               | Werte formatiert wie heute                                                                                    |
| 4 statische Diagramme (PNG), Gruppierung 2 + 1 + 1     | 4 Chart.js-Diagramme, gleiche Gruppierung                                   | neu: Tooltips, umschaltbare Legende, Zoom in der Zeitreihe, Drill-down per Klick, Tabellenansicht je Diagramm |
| Diagramm als PNG exportieren                           | PNG-Export im Browser, CSV aus der Tabellenansicht                          |                                                                                                               |
| Berichtstabelle, „Weitere laden", Gruppierung          | HTML-Tabelle, „Weitere laden" per htmx, sortierbare Spalten                 | Keyset-Pagination bleibt                                                                                      |
| Bericht-Detaildialog                                   | `<dialog>`-Element mit Schließen-Knopf **und** Esc                          | der Fehler „lässt sich nicht schließen" kann nicht wieder auftreten                                           |
| CSV-Export (nur geladene Seiten)                       | CSV-Export des **gesamten** gefilterten Bestands, seitenweise gestreamt     | Verbesserung: kein Speicherproblem, weil gestreamt                                                            |
| Sendequellen mit PTR/Dienst                            | HTML-Tabelle                                                                |                                                                                                               |
| Einstellungen: Konten anlegen/testen/löschen           | Formular + Liste                                                            | Validierung serverseitig (Logik aus `AccountForm.Validate` übernehmen)                                        |
| Ordner-Picker (`SelectEntry`)                          | `<input list>` mit `<datalist>`, Knopf „Ordner auflisten"                   |                                                                                                               |
| Ersteinrichtung (3 Schritte)                           | `/einrichtung`                                                              |                                                                                                               |
| Sync-Knopf mit Fortschritt und Abbruch                 | Knopf + Fortschrittsanzeige über SSE                                        | Sync läuft serverseitig weiter, auch beim Seitenwechsel                                                       |
| Leerzustände, Fehlerbanner mit technischen Details     | Partials, Details in `<details>`                                            |                                                                                                               |
| Glossar-Dialog, „?"-Knöpfe                             | Glossarseite + echte Tooltips (`popover`-Attribut, per Tastatur erreichbar) |                                                                                                               |
| —                                                      | **neu:** Datei-Import per Upload/Drag & Drop (E-6)                          |                                                                                                               |

## 9. Nötige Erweiterungen außerhalb der Präsentationsschicht

1. **Sync-Fortschritt** — `syncreports.UseCase.SyncAccount` liefert heute nur ein
   Endergebnis. Ergänzung: optionaler Fortschritts-Callback (verarbeitet /
   neu / übersprungen / fehlerhaft).
2. **Sync als serverseitiger Auftrag** — neues Paket `internal/app/syncjob`:
   höchstens ein Lauf gleichzeitig, Abbruch per Kontext, Fortschritt für
   mehrere Zuhörer. Liegt in der Anwendungsschicht, weil der geplante
   Hintergrund-Sync aus AP 7 dieselbe Sperre braucht.
3. **Import aus Bytes** — `importfiles.UseCase` arbeitet heute mit Dateipfaden.
   Ergänzung `ImportData(ctx, filename, data)`; die interne Zerlegung (`extractAttachments`) arbeitet bereits mit Bytes.
4. **Sperrbarer Schlüsselspeicher** — `newCredentialStore()` fragt heute beim
   Start auf der Konsole nach der Master-Passphrase. Ergänzung: ein
   `account.CredentialStore`, der bis zum Entsperren
   `account.ErrCredentialStoreLocked` liefert; die Oberfläche leitet dann auf
   `/entsperren`.
5. **Diagramm-Port zurückbauen** — `analysis.ChartRenderer`,
   `internal/infra/charts`, `exportdata.WriteChartPNG` und go-chart entfallen.
   `analysis.DailyVolume/SourceVolume/Heatmap` bleiben als Daten für die
   JSON-Endpunkte; `HeatmapCell` bekommt `Total` (Abschnitt 6a). Eine ADR in
   `docs/` hält fest, warum der in `IMPLEMENTIERUNG.md` 8.2 vorgesehene
   „Adapter-Tausch" hier zum Rückbau wird: Diagramme sind im Web reine
   Darstellung im Browser, der Server liefert nur Daten.
6. **Drill-down-Filter** — `report.Query` kennt bereits Zeitraum, Domain,
   Quell-IP und Disposition. Zu prüfen in M2: ob „Berichte einer Quelle an
   einem Tag" allein mit Zeitraum + Quell-IP korrekt abgebildet ist (Zuordnung
   über `date_begin`, siehe AP 6).

## 10. Arbeitspakete

Größenangaben relativ: S (klein), M (mittel), L (groß).

### M0 — Entscheidungen und Durchstich (S)

- [x] Entscheidungen E-1 bis E-8 bestätigt — implizit durch Umsetzung der
  jeweiligen Empfehlung bestätigt (Chart.js, Strg+C/SIGTERM,
  zufälliger Port, Datei-Import folgt M4, vendorierte JS-Bündel,
  `chromedp`-Rauchtest folgt M2); nicht einzeln nachverhandelt.
- [x] `internal/web` mit Server, Einmal-Anmeldung, Host-Prüfung, Browserstart
- [x] Eine Seite: Übersicht mit Kennzahlen-Kacheln aus `statistics.UseCase`
- [x] **Chart.js-Durchstich:** Heatmap (Matrix-Plugin) und Zeitreihe mit Zoom
  aus echten Daten, Tooltip, ein Drill-down-Klick — unter der geplanten
  Content-Security-Policy ohne `'unsafe-inline'`
- [x] Startpfad hinter Flag (`dmarc-analyzer web`), Fyne bleibt Standard

**Fertig wenn:** Ein Start öffnet den Browser, zeigt echte Kennzahlen und zwei
interaktive Diagramme, und eine Anfrage ohne Sitzung oder mit fremdem `Host`
wird abgewiesen. Trägt die Chart.js-Plugin-Kombination nicht, fällt hier die
Entscheidung für die Rückfallebene ECharts (E-2). — **Erreicht.**

### M1 — Grundgerüst (M)

- [x] Layout, Navigation, Kopfzeile, Theme hell/dunkel als CSS-Variablen —
  Navigation aktuell mit vier Platzhalter-Seiten (Berichte, Sendequellen,
  Glossar, Einstellungen; Inhalt folgt M2/M3), Theme aus M0 unverändert.
- [x] Filterleiste mit Werten in der URL — wirkt auf Übersicht
  (Kennzahlen, beide Diagramme inkl. Drill-down-Ziele); Berichte/Quellen
  übernehmen denselben Filter, sobald sie in M2 echten Inhalt bekommen.
- [x] Sicherheits-Middleware vollständig (Abschnitt 5), CSRF, CSP —
  `requireCSRF` (Origin/Sec-Fetch-Site + Token) ist bereits um den
  gesamten geschützten Routenbaum gelegt, auch wenn noch kein Formular
  es braucht (erster Verbraucher: Konten-Formular in M3).
- [x] Einzelinstanz (`instance.json`), zweiter Start öffnet bestehende Instanz
- [x] Sauberes Herunterfahren per Strg+C/SIGTERM (E-3, `signal.NotifyContext` + `http.Server.Shutdown`), `--kein-browser`, `--adresse`, `--entwicklung`
- [ ] `i18n` und Glossar-Daten verschoben; Leerzustand- und Fehler-Partials —
  **zurückgestellt auf M5.** Ein echter Verschub jetzt würde
  `internal/ui` (bis M3 Standard-Oberfläche, siehe E-5) die Pakete unter
  den Füßen wegziehen; ein `internal/ui` → `internal/web`-Import wäre
  zudem eine verbotene Abhängigkeitsrichtung (AGENTS.md). Bis dahin
  bleiben die wenigen bisher gebrauchten Texte (Navigation,
  Filterleiste) direkt in `internal/web` als deutsche Literale — reale
  Doppelpflege beginnt erst, wenn tatsächlich beide Oberflächen aus
  derselben i18n-Quelle laufen müssten, und das ist nicht der Plan
  (E-5: Umschalten, nicht Parallelbetrieb). Leerzustand-/Fehler-Partials
  ebenfalls zurückgestellt: die einzige bisherige "leere" Situation sind
  die M1-Platzhalterseiten selbst.
- [x] htmx, Chart.js und Plugins eingebunden, Versionen und Lizenzen in `docs/DEPENDENCIES.md` gepinnt

**Fertig wenn:** Navigation zwischen leeren Seiten funktioniert in hell und
dunkel, ein zweiter Start öffnet nur einen weiteren Tab, und Strg+C/SIGTERM
beendet den Prozess sauber (offene Anfragen fertig, `instance.json` entfernt).
— **Erreicht** (Sichtprüfung in echten Browsern gemäß M6 steht noch aus,
siehe dortiger Vorbehalt zu fehlendem Display in dieser Umgebung).

### M2 — Lesende Ansichten (L)

- [x] JSON-Endpunkte für alle vier Diagramme inkl. Drill-down-URLs; `HeatmapCell.Total`
- [x] `charts.js`: vier Diagramme nach Abschnitt 6a, Farben aus CSS-Variablen,
  Neuzeichnen bei Wechsel hell/dunkel; Palette **nicht** gegen
  `scripts/validate_palette.js` der `dataviz`-Skill geprüft — die Skill
  war in dieser Session nicht verfügbar (nur `customize-opencode`
  geladen). Wiederverwendet wurden ausschließlich die bereits in M0/M1
  validierten CSS-Variablen (`--status-good/-warning/-critical`,
  `--primary`); keine neuen Farbwerte erfunden. Vor einem Release: Skill
  laden und Validierung nachholen.
- [x] Übersicht vollständig: Kacheln, Trend, vier Diagramme in der bestehenden
  Gruppierung (Zeitreihe+Donut nebeneinander, Top-Sendequellen und Heatmap
  je volle Zeile), Tooltips, Legende, Zoom, Drill-down, Tabellenansicht je
  Diagramm
- [x] Berichtstabelle mit Sortierung (Organisation/Domain/Zeitraum, klickbare
  Spaltenköpfe), Gruppierung (keine/Domain/Organisation), „Weitere laden"
  (htmx, Keyset-Cursor), Detailansicht (Metadaten/Richtlinie/Sendequellen)
  und den Drill-down-Filtern (Quell-IP, Disposition, Tag als von/bis);
  Erweiterung 9.6 geprüft — Überlappungs-Semantik (`date_begin < bis AND
  date_end > von`) bestätigt am realen Adapter, kein Anpassungsbedarf.
  **Nicht umgesetzt:** Detailansicht ist eine eigene Seite, kein
  `<dialog>`-Overlay per htmx (Abschnitt 8 sah ein Dialog-Element vor) —
  bewusste Scope-Entscheidung dieser Session, um M2 in vertretbarer Zeit
  abzuschließen; funktional vollständig (Zurück-Link, eigene URL,
  Tastatur-/Screenreader-freundlich), nur ohne Modal-Politur. Kann in M6
  nachgerüstet werden.
- [x] Sendequellen-Tabelle (Quell-IP, Nachrichten, Pass-Rate, PTR-Hostname,
  erkannter Dienst; sortierbar nach Quell-IP/Nachrichten; „Weitere laden")
- [x] Glossarseite und Begriffs-Tooltips — Daten liegen als
  `internal/web/glossary` (Kopie aus `internal/ui/glossary/terms.go`,
  siehe M1-Begründung zu i18n/Glossar-Verschub); "?"-Links auf der
  Übersicht neben Kacheln/Diagrammtiteln verweisen auf `/glossar#<slug>`.
- [ ] `chromedp`-Rauchtest (E-8) — **zurückgestellt.** In dieser
  Sandbox ist kein Chrome/Chromium installiert (`which google-chrome
  chromium chromium-browser` liefert nichts, keine Chrome-Installation
  unter `/Applications`); ein Rauchtest ließe sich hier nicht schreiben
  UND verifizieren. Alles, was der Rauchtest zusätzlich zu den
  bestehenden `httptest`-Handlertests geprüft hätte (Chart.js zeichnet
  wirklich, keine Konsolenfehler, ein echter Klick navigiert), ist
  stattdessen manuell mit dem gebauten Binary + `curl` gegen importierte
  Testreports verifiziert worden (Login, Übersicht, Berichte inkl.
  Sortierung/Filter/Paginierung, Sendequellen, Detailansicht) — das
  ersetzt aber keinen echten Browser-Test. Nachholen, sobald eine
  Umgebung mit Chrome verfügbar ist (lokale Entwicklungsmaschine oder
  CI-Runner mit `chromium`/`google-chrome` vorinstalliert).

**Fertig wenn:** Alle lesenden Funktionen der Fyne-Oberfläche sind im Browser
vorhanden (Tabelle in Abschnitt 8), jeder Klick auf ein Diagramm führt zu den
passenden Berichten, und bei 100.000 Records bleiben Berichtstabelle und
Diagramme flüssig (Heatmap über ein Jahr: 10 Quellen × 365 Tage). —
**Funktional erreicht**, manuell mit importierten Testreports verifiziert
(kein Fyne-Feature aus Abschnitt 8 fehlt mehr außer dem Dialog-Overlay,
siehe oben). **Nicht verifiziert:** Performance bei 100.000 Records —
Berichts-/Sendequellen-Abfragen laufen über dieselben, bereits in AP 2/4
mit realistischen Datenmengen getesteten Keyset-Repositories
(`internal/infra/sqlite`, siehe `reportperf_test.go`), die Web-Handler
selbst laden nie mehr als eine Seite (`Limit: 50`) — ein eigener
Lasttest auf HTTP-Ebene stand in dieser Session nicht an. Echter
Browser-Rauchtest fehlt (siehe chromedp-Punkt oben).

### M3 — Schreibende Abläufe (L)

- [ ] Erweiterungen 9.1, 9.2, 9.4 umgesetzt und getestet
- [ ] Einstellungen: Konten anlegen, testen, löschen, Ordner auflisten
- [ ] Ersteinrichtung in drei Schritten
- [ ] Sync mit Fortschritt (SSE) und Abbruch
- [ ] Entsperr-Seite für den Datei-Schlüsselspeicher
- [ ] **Umschalten:** `dmarc-analyzer` ohne Argumente startet die Web-Oberfläche

**Fertig wenn:** Ein Nutzer richtet ohne Dokumentation im Browser ein Konto ein,
wählt einen Unterordner, gleicht ab und sieht seine Reports — derselbe
Abnahmesatz wie AP 5.

### M4 — Import und Export (M)

- [ ] Erweiterung 9.3; Upload mit Drag & Drop, Ergebnis-Anzeige (neu /
  übersprungen / fehlerhaft)
- [ ] CSV-Export des gesamten gefilterten Bestands, gestreamt
- [ ] Diagramm-Export als PNG (Browser) und CSV (Tabellenansicht)

**Fertig wenn:** Eine `.zip` mit mehreren Reports lässt sich per Drag & Drop
importieren, und ein Export von 100.000 Records läuft ohne spürbaren
Speicheranstieg.

### M5 — Umstellung und Rückbau (M)

- [ ] `internal/ui` und `cmd/dmarc-analyzer/cmd_gui.go` gelöscht; Fyne aus `go.mod`
- [ ] Erweiterung 9.5 (Diagramm-Port, `internal/infra/charts`, go-chart entfernt)
- [ ] Build mit `CGO_ENABLED=0`; `task release` als Cross-Compile für
  macOS (arm64, amd64), Windows (amd64), Linux (amd64, arm64) mit Checksummen
- [ ] Windows-Build ohne Konsolenfenster (`-H windowsgui`), Log in Datei —
  Verhalten beim Doppelklick prüfen
- [ ] macOS: Verhalten einer Nicht-Cocoa-Binärdatei im `.app`-Bündel prüfen (Dock-Symbol, „reagiert nicht"-Anzeige);
  falls nötig `LSUIElement`
- [ ] Taskfile: `package:*` und `fyne`-Installation entfernt, `run` öffnet den Browser
- [ ] CI: Cross-Compile-Schritt ergänzt
- [ ] Dokumentation nach Abschnitt 12 angepasst

**Fertig wenn:** `go list -deps ./... | grep fyne` ist leer, `task check` ist grün,
und die Release-Binärdateien für alle Plattformen entstehen auf einem Rechner.

### M6 — Feinschliff (S–M)

- [ ] Barrierefreiheit: vollständige Tastaturbedienung, sichtbarer Fokus,
  Kontraste, `prefers-reduced-motion`, Tabellenansichten der Diagramme
- [ ] Sichtprüfung in Safari, Firefox, Chrome und Edge, jeweils hell und dunkel
- [ ] Rauchtest um die schreibenden Abläufe erweitern, falls sich dort Fehler häufen

**Fertig wenn:** Alle Abläufe sind ohne Maus bedienbar und in allen vier
Browsern ohne Darstellungsfehler.

### Reihenfolge

```
M0 ──▶ M1 ──▶ M2 ──▶ M3 (Umschalten) ──▶ M4 ──▶ M5 (Rückbau) ──▶ M6 ──▶ AP 7
```

M4 kann parallel zu M3 laufen. Bis einschließlich M2 bleibt Fyne der Standard.

## 11. Tests

| Ebene                | Vorgehen                                                                                                                                                                                                                  |
|----------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Sicherheit           | `httptest`: ohne Sitzung → 401; falscher `Host` → 421/400; POST ohne CSRF-Token oder mit fremdem `Origin` → 403; verbrauchter/abgelaufener Einmal-Code → abgewiesen; Passwort taucht in keiner Antwort und keinem Log auf |
| Handler              | `httptest` mit den bestehenden handgeschriebenen Fakes (aus `internal/ui/*_test.go` übernommen): Statuscodes, Weiterleitungen (keine Konten → Ersteinrichtung), gerenderte Kerninhalte, Leer- und Fehlerzustände          |
| Templates            | Jede Seite wird in Tests gerendert und als HTML geparst — ein Template-Fehler fällt im Test auf, nicht erst im Browser                                                                                                    |
| Diagrammdaten        | JSON-Endpunkte mit `httptest`: leere Daten, ein Wert, viele Werte; kein `NaN`/`Inf` im JSON (bricht `encoding/json`); Drill-down-URLs enthalten die richtigen Filter; Labels angereichert                                 |
| Diagramme im Browser | `chromedp`-Rauchtest (E-8): vier Diagramme gezeichnet, keine Konsolenfehler, ein Klick führt zur gefilterten Berichtsseite. `charts.js` bleibt bewusst dünn, damit die meiste Logik in Go getestet wird.                  |
| Lebenszyklus         | Einzelinstanz-Erkennung, verwaister `instance.json`, sauberes Herunterfahren bei SIGTERM (offene Anfragen fertig, `instance.json` entfernt)                                                                                                                       |
| Anwendungsschicht    | neue Tests für Fortschritts-Callback, `syncjob` (nur ein Lauf gleichzeitig, Abbruch), `ImportData`, gesperrter Schlüsselspeicher                                                                                          |
| Sichtprüfung         | zusätzlich manuell je Meilenstein, hell und dunkel                                                                                                                                                                        |

Die bestehenden Konventionen gelten weiter: `testify/require`, handgeschriebene
Fakes, `-race`, deutsche Kommentare.

## 12. Dokumentation

| Datei                  | Anpassung                                                                                                                                                                                                                                  |
|------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `FEATURES.md`          | Fyne-Vorgabe durch „eingebettete Web-Oberfläche im Browser" ersetzen — **eigene Vorgabe des Projekts, Änderung bitte bestätigen**                                                                                                          |
| `IMPLEMENTIERUNG.md`   | Abschnitte 3 (Stack), 5 (Struktur), 8.2 (ChartRenderer), 10 (UI-Konzept statt Fyne-Praxis), 12 (Tests), 13 (Taskfile)                                                                                                                      |
| `UMSETZUNGSPLAN.md`    | Verweis auf diesen Plan vor AP 7; AP 7 anpassen: Desktop-Benachrichtigung → Browser-Benachrichtigung bei offenem Tab, Packaging ohne `fyne package`                                                                                        |
| `docs/DEPENDENCIES.md` | htmx, Chart.js, `chartjs-chart-matrix`, `chartjs-plugin-zoom` (Versionen, Lizenzen, Kompatibilität) und `chromedp` aufnehmen; Fyne und go-chart entfernen                                                                                  |
| `docs/`                | ADR „Web-Oberfläche statt Fyne" und ADR „Interaktive Diagramme mit Chart.js statt ChartRenderer-Port"                                                                                                                                      |
| `AGENTS.md`            | Fyne-Abschnitte entfernen; neue Regeln: keine Inline-Skripte (CSP), jede Zustandsänderung per POST mit CSRF, Templates immer im Test rendern, Diagrammlogik (Aggregation, Filter, Drill-down-Ziele) gehört nach Go, nicht nach `charts.js` |
| `README.md`            | Start, Browserverhalten, Beenden (Strg+C/SIGTERM), `--kein-browser`, Sicherheitsmodell                                                                                                                                                                      |
| `CHANGELOG.md`         | je Meilenstein ein Eintrag                                                                                                                                                                                                                 |

## 13. Risiken

| Risiko                                                                              | Gegenmaßnahme                                                                                                                                                            |
|-------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Server läuft unbemerkt weiter, wenn nur der Browser-Tab geschlossen wird (Desktop-Nutzung, kein Docker) | Bewusst in Kauf genommen (E-3): kein Auto-Ende mehr. Abgemildert durch die Konsolenausgabe beim Start ("Strg+C zum Beenden") und die Einzelinstanz-Erkennung — ein zweiter Doppelklick startet keinen weiteren Prozess. Falls sich das als echtes Ärgernis zeigt: in M6 nachrüsten, ohne den Docker-Betrieb zu berühren (reiner Zusatz, kein Ersatz für SIGTERM). |
| Andere Webseiten oder lokale Benutzer greifen auf den Server zu                     | Abschnitt 5 vollständig in M1, mit Tests                                                                                                                                 |
| Kein Browser verfügbar (Server, SSH)                                                | `--kein-browser`, Adresse auf der Konsole; CLI bleibt unverändert                                                                                                        |
| Doppelklick-Start verhält sich je Betriebssystem unerwartet (Konsolenfenster, Dock) | Prüfpunkte in M5                                                                                                                                                         |
| Funktionen gehen beim Übertrag verloren                                             | Paritätstabelle (Abschnitt 8) als Abnahmeliste für M3                                                                                                                    |
| Chart.js-Plugins passen nicht zur eingebundenen Chart.js-Version oder zur CSP       | Heatmap und Zoom zuerst im Durchstich (M0); Versionen gemeinsam pinnen und nur gemeinsam aktualisieren; Rückfallebene ECharts betrifft nur `charts.js` und die JSON-Form |
| `<canvas>` ist für Screenreader unsichtbar                                          | Kurzbeschreibung und Tabellenansicht je Diagramm, Drill-down auch als Links                                                                                              |
| Große Heatmap (viele Tage) wird unlesbar oder langsam                               | Zoom/Verschieben; Zeitraum-Vorgabe; Grenzwert im Rauchtest (10 × 365 Zellen)                                                                                             |
| Logik wandert unbemerkt ins JavaScript und bleibt ungetestet                        | Regel in `AGENTS.md`: Aufbereitung und Drill-down-Ziele kommen fertig vom Server                                                                                         |
| Die Oberfläche wird im Browser nicht mehr „nativ" wirken                            | bewusst in Kauf genommen                                                                                                                                                 |

## 14. Offene Fragen

1. Entscheidungen E-1, E-4 bis E-8 — gelten die Empfehlungen? (E-2 ist mit
   Chart.js, E-3 mit Strg+C/SIGTERM festgelegt.)
2. Darf `FEATURES.md` entsprechend geändert werden?
3. Soll die Oberflächensprache weiterhin nur Deutsch sein (die Struktur mit
   `i18n` würde Englisch später erlauben)?
4. Soll der Server auf Wunsch auch im Netz erreichbar sein — sei es im
   Heimnetz (NAS) oder als Docker-Image (Abschnitt 5, „Spannung mit dem
   geplanten Docker-Einsatz")? Dieser Plan baut nur das Loopback-Modell
   (nur derselbe Rechner); Netzwerk-Erreichbarkeit bräuchte echte Anmeldung
   (Benutzername/Passwort statt Einmal-Link) und TLS.
5. Ist die Docker-Verpackung Teil **dieser** Migration (dann müsste M5/M6 um
   ein `Dockerfile` und das in Frage 4 skizzierte Sicherheits-Nachtrag
   ergänzt werden) oder ein eigener, späterer Plan? Empfehlung: eigener,
   späterer Plan — diese Migration liefert zunächst die lokale
   Web-Oberfläche; Docker braucht ein anderes Bedrohungsmodell (Frage 4)
   und sollte nicht nebenbei mitentschieden werden.
