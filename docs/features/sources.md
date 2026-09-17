# Sendequellen

`/quellen` aggregiert alle importierten Records nach Quell-IP
(`internal/domain/sources`, `internal/app/sourcestats`) — Volumen,
Pass-Rate, zeitliche Ein-/Ausgrenzung, plus Anreicherung.

## Aggregation

Filterbar nach Zeitraum und Domain, sortierbar nach Volumen (Vorgabe,
größte Quelle zuerst) oder nach Quell-IP. Wie bei der Berichtstabelle:
Keyset-Pagination statt Offset, Filter stehen in der URL.

## Anreicherung

Zusätzlich zu den reinen Report-Daten wird jede Quell-IP angereichert
(`internal/infra/sourceinfo.Enricher`, PTR-/rDNS-Auflösung plus Erkennung
bekannter Versanddienste, z. B. "Google Workspace"). Eine nicht auflösbare
PTR oder ein nicht erkannter Dienst sind normale, erwartete Ausgänge (die
Oberfläche zeigt dann "—" bzw. die reine IP-Adresse), keine Fehlerbedingung.
Ergebnisse werden intern gecacht — PTR-Auflösung ist Netzwerk-I/O und
ändert sich selten, ein wiederholter Lookup derselben IP lohnt sich nicht.
Dieselbe Anreicherung fließt zusätzlich in die Top-Sendequellen- und
Heatmap-Diagramme des Dashboards ein (dieselben erkannten Labels).

## CSV-Export

`GET /export/quellen.csv` exportiert den gesamten gefilterten Bestand
(nicht nur die aktuell angezeigte Seite).
