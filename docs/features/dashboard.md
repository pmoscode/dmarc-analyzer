# Übersicht (Dashboard)

`/` zeigt die zentralen Kennzahlen für einen wählbaren Zeitraum
(`internal/app/statistics.UseCase.Dashboard`), gefiltert nach Zeitraum und
Domain (Filterleiste, Werte stehen in der URL).

## Kennzahlen

- **Nachrichten gesamt** — Summe aller `Record.Count` im Zeitraum (ein
  Record mit Count=50 sind 50 Nachrichten von dieser Quelle, nicht ein
  Report).
- **DMARC-Pass-Rate** — Anteil der Nachrichten, die DKIM- oder
  SPF-aligned sind.
- **DKIM-/SPF-Alignment-Rate** — getrennt ausgewiesen, weil ein
  Nachrichtenstrom mit hoher Pass-Rate trotzdem eine schwache Alignment-Rate
  bei genau einem der beiden Mechanismen haben kann (relevant für die
  Fehlersuche).
- **Anzahl unterschiedlicher Quell-IPs**.
- **Trend gegenüber der Vorperiode** — Pass-Rate der aktuellen Periode
  minus Pass-Rate der unmittelbar davorliegenden, gleich langen Periode.

## Diagramme

Alle vier Diagramme laden ihre Daten selbst per JavaScript von
`/api/diagramme/*` nach (JSON, aufbereitet in Go — `charts.js` bleibt
bewusst dünn) und reagieren auf Hell-/Dunkelmodus über CSS-Variablen:

- **Nachrichtenvolumen pro Tag**, gestapelt nach Pass/Fail, mit Zoom.
- **Verteilung nach Disposition** (none/quarantine/reject).
- **Top-Sendequellen nach Volumen**, eingefärbt nach Pass-Rate.
- **Sendequelle × Tag** als Heatmap, Farbe = Pass-Rate, Tooltip mit
  Quelle, Tag, Pass-Rate und Nachrichtenzahl.

Jedes Diagramm-Element (Tag, Quelle, Zelle, Segment) ist klickbar und öffnet
die Berichtstabelle mit passend vorbelegtem Filter (Drill-down). Einzelne
Diagramme lassen sich außerdem direkt als PNG oder CSV exportieren.
