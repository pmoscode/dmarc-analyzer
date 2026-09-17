# ADR 0002: Chart.js im Browser statt `analysis.ChartRenderer`-Port

- Status: angenommen
- Datum: 2026-09-17 (Migrationsentscheidung), umgesetzt in `MIGRATIONSPLAN.md`
  M0 (Durchstich) bis M5 (Port entfernt)

## Kontext

Vor der Web-Migration lieferte `internal/domain/analysis.ChartRenderer`
einen Port, den `internal/infra/charts` gegen
[`go-chart/v2`](https://github.com/wcharczuk/go-chart) implementierte
(`IMPLEMENTIERUNG.md` Abschnitt 8.2: "ChartRenderer als Port", gedacht als
austauschbarer Adapter — z. B. später gegen native, interaktive
Fyne-Widgets). In der Praxis:

- Diagramme waren serverseitig gerenderte `image.Image`/PNGs ohne Tooltip,
  ohne Ein-/Ausblenden von Datenreihen, ohne Zoom, ohne Klick-Drilldown.
- `go-chart` kennt keinen Heatmap-Typ — `internal/infra/charts/heatmap.go`
  zeichnete die Sendequelle-×-Tag-Matrix manuell über `image/draw`.
- Die PNGs hatten einen fest weißen Hintergrund (`flattenOnWhite`), weil
  `go-chart` keine Transparenz für den umgebenden Kartenhintergrund anbot —
  im dunklen Systemmodus erschienen dadurch helle Kästen um jedes Diagramm.
- `internal/infra/charts/renderer.go` enthielt mehrere reine
  Umgehungslösungen für go-chart-Eigenheiten (Textumbruch, Ein-Wert-Donut,
  transparente Ränder) ohne fachlichen Mehrwert.

Mit dem Umstieg auf eine Web-Oberfläche (ADR 0001) fällt die ursprüngliche
Motivation für den Port ("später gegen native, interaktive Fyne-Widgets
tauschen") weg: Im Browser sind Diagramme grundsätzlich Client-Seite, nicht
serverseitig gerenderte Bilder.

## Entscheidung

Diagramme entstehen vollständig im Browser mit
[Chart.js](https://www.chartjs.org/) (`internal/web/static/charts.js`),
ergänzt um die Plugins `chartjs-chart-matrix` (Heatmap-Diagrammtyp, den
Chart.js selbst nicht mitbringt) und `chartjs-plugin-zoom` (Zoom/Verschieben
in der Zeitreihe). Der Server liefert ausschließlich aufbereitete
JSON-Daten über eigene Endpunkte (`/api/diagramme/*`); alle
Aggregations-, Filter- und Drill-down-Entscheidungen fallen in Go
(`internal/app/statistics`, `internal/domain/analysis`), `charts.js` bleibt
bewusst dünn (reine Darstellung, siehe `AGENTS.md`: "Diagrammlogik gehört
nach Go").

Damit entfallen vollständig:

- `analysis.ChartRenderer` (Port, `internal/domain/analysis/charts.go`) —
  die reinen Datentypen `DailyVolume`, `SourceVolume`, `Heatmap`,
  `HeatmapCell` bleiben unverändert erhalten und werden jetzt direkt zu
  JSON serialisiert statt an einen Bild-Renderer übergeben zu werden.
  `HeatmapCell` bekam dafür in M2 zusätzlich ein `Total`-Feld (Nachrichten-
  zahl je Zelle, für die Browser-Tooltips).
- `internal/infra/charts` (die go-chart-Implementierung des Ports,
  4 Dateien inkl. der manuellen Heatmap-Zeichnung).
- `exportdata.WriteChartPNG` (PNG-Kodierung eines gerenderten Diagramm-Bilds
  — ihr einziger Aufrufer war der jetzt gelöschte Fyne-Dashboard-PNG-Export
  in `internal/ui/dashboard/view.go`).
- `github.com/wcharczuk/go-chart/v2` aus `go.mod`.

Diagramm-Export als Datei (PNG/CSV) bleibt als Funktion erhalten, jetzt aber
rein clientseitig: `chart.toBase64Image()` für PNG, ein kleines
JavaScript-Objekt aus denselben Daten wie die zugehörige Tabellenansicht
für CSV (siehe `internal/web/static/charts.js`, `chartExports`-Registry) —
kein serverseitiger Bild-Kodierungsschritt mehr nötig.

Erwogene Alternative war [Apache ECharts](https://echarts.apache.org/):
bringt Heatmap, Zoom und Bild-Export bereits eingebaut mit, ist aber mit
≈1 MB deutlich größer und hat ein eigenes Theme-System, das gegen die
bestehende CSS-Variablen-Palette (`AGENTS.md`: "Diagrammfarben folgen der
`dataviz`-Skill-Referenzpalette") hätte abgeglichen werden müssen
(`MIGRATIONSPLAN.md` Entscheidung E-2). Chart.js + zwei kleine, gezielte
Plugins wurde als schlankere Lösung vorgezogen; ECharts bleibt die
dokumentierte Rückfallebene, falls sich die Plugin-Kombination als nicht
tragfähig erweist — betroffen wären dann nur `charts.js` und die
JSON-Form der Endpunkte, nicht die Go-Seite.

## Konsequenzen

**Vorteile:**

- Echte Interaktivität: Tooltips, umschaltbare Legende, Zoom/Verschieben in
  der Zeitreihe, Klick-Drilldown auf Tag/Quelle/Heatmap-Zelle/Disposition —
  jeweils zu einer gefilterten Berichtsansicht (Server liefert die
  Ziel-URLs fertig mit).
- Diagramme passen sich automatisch an hell/dunkel an (Chart.js liest die
  Farben aus denselben CSS-Variablen wie die restliche Oberfläche, siehe
  `AGENTS.md`-Abschnitt zur Diagrammfarben-Palette) — kein fest weißer
  Hintergrund mehr.
- Deutlich weniger Code und keine reinen Bibliotheks-Umgehungslösungen
  (die gesamten go-chart-Workarounds sind mit dem Paket verschwunden).
- Kleinerer, CGO-freier Server-Build (siehe ADR 0001) — die
  Diagramm-Bibliothek liegt jetzt als minifizierte JavaScript-Datei im
  Repository (`internal/web/static/vendor/`, siehe
  `docs/DEPENDENCIES.md`), nicht mehr als Go-Abhängigkeit.

**Nachteile / bewusst in Kauf genommen:**

- Ein `<canvas>` ist für Screenreader unsichtbar — abgemildert durch eine
  Kurzbeschreibung (`aria-label`) und eine zuschaltbare Tabellenansicht je
  Diagramm (`<details><summary>Als Tabelle anzeigen</summary>...`), Drill-
  down-Ziele stehen zusätzlich als normale Links in dieser Tabelle.
- Mit Chart.js steckt echte Logik im JavaScript (Datenzuordnung, Farben,
  Klick-Ziele), die reine `httptest`-Handler-Tests nicht sehen — dafür ist
  laut `MIGRATIONSPLAN.md` Abschnitt 11 ein `chromedp`-Rauchtest (E-8)
  vorgesehen (vier Diagramme gezeichnet, keine Konsolenfehler, ein
  Drill-down-Klick funktioniert). In den Entwicklungsumgebungen, in denen
  M2–M5 entstanden, war kein Chrome/Chromium verfügbar — der Rauchtest
  bleibt bislang ungeschrieben und ist als offener Punkt in
  `MIGRATIONSPLAN.md` vermerkt, nicht Teil dieser Entscheidung selbst.
- Ein bereits einmal aufgetretener, undokumentiert gebliebener Fallstrick
  mit `responsive: true`/`maintainAspectRatio: false` (Canvas ohne
  Wrapper mit fester Höhe wächst unendlich) ist inzwischen in `AGENTS.md`
  festgehalten — reine Chart.js-API-Falle, kein grundsätzliches
  Gegenargument zur Entscheidung.

## Alternativen

- **`analysis.ChartRenderer`-Port beibehalten, nur Diagramme im Browser per
  Bild-Tag einbetten:** hätte die eigentlichen Vorteile (Tooltips, Zoom,
  Klick-Drilldown, automatische Hell/Dunkel-Anpassung) nicht erreicht —
  wäre nur ein PNG im `<img>`-Tag statt im Fyne-Fenster gewesen.
- **Apache ECharts:** siehe oben, als Rückfallebene dokumentiert, nicht
  gewählt (Größe, eigenes Theme-System).
- **D3.js (volle Kontrolle, kein fertiges Diagramm-Framework):** nicht
  ernsthaft erwogen — deutlich mehr Code für dieselben vier Diagrammtypen,
  ohne die Reifegrad-/Tooltip-/Zoom-Bausteine, die Chart.js bereits fertig
  mitbringt.
