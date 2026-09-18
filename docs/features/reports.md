# Berichte

## Speicherung

Jeder importierte DMARC-Aggregate-Report wird vollständig oder gar nicht
gespeichert (`report.Repository.Save` läuft in einer Transaktion): Metadaten (Absender-Organisation, Berichtszeitraum,
veröffentlichte Policy) plus
alle enthaltenen Records (Quell-IP, Nachrichtenzahl, Disposition,
DKIM-/SPF-Ergebnisse, Reasons) sowie der komplette Rohbericht (für spätere
Nachvollziehbarkeit). Deduplizierung über einen `UNIQUE`-Index auf (`org_name`, `report_id`, `date_begin`) — derselbe
Report von zwei
verschiedenen Wegen importiert (z. B. per IMAP und zusätzlich per
Datei-Upload) wird nur einmal gespeichert.

## Berichtstabelle (`/berichte`)

Filtert, sortiert und gruppiert serverseitig in SQL (keine Offset-, sondern
Keyset-Pagination — bleibt auch bei vielen Reports schnell):

- **Filter**: Zeitraum, Domain, Absender-Organisation, Quell-IP,
  Disposition.
- **Sortierung**: nach Berichtsbeginn, Absender-Organisation oder Domain.
- **Gruppierung** (wirkt als zusätzlicher, primärer Sortierschlüssel):
  nach Domain, nach Organisation oder nach Quell-IP.

Filter stehen in der URL — Lesezeichen und der Zurück-Knopf des Browsers
funktionieren.

## Berichtsdetail (`/berichte/{id}`)

Lädt einen einzelnen Report vollständig, inklusive aller Records — die
Berichtstabelle selbst lädt bewusst keine Records (eine Zeile pro Report,
nicht pro Record).

## CSV-Export

`GET /export/berichte.csv` exportiert den **gesamten gefilterten
Bestand** (nicht nur die aktuell angezeigte Seite) als CSV (`internal/app/exportdata`). Einzelne Dashboard-Diagramme
lassen sich
zusätzlich direkt als PNG oder CSV exportieren.
