# IMAP-Abholung und Import

## Inkrementeller Sync

`internal/app/syncreports.UseCase.SyncAccount` verbindet sich mit dem
konfigurierten IMAP-Postfach (`internal/infra/imap`) und holt **nur neue**
Nachrichten seit dem letzten Lauf ab (`sync.State`: zuletzt gesehene
UID + UIDVALIDITY). Ändert sich die UIDVALIDITY des Postfachs (z. B. nach
einer Postfach-Migration beim Provider), erkennt das Protokoll das selbst
und ein vollständiger Rescan läuft erneut — der `UNIQUE`-Index auf
`reports` (`org_name`, `report_id`, `date_begin`) verhindert dabei
Duplikate.

Abholen und Parsen laufen nebenläufig, Schreiben nach SQLite seriell —
Details in [`../architecture.md`](../architecture.md#sync-pipeline-internalappsyncreports).

Ausgelöst wird ein Sync-Lauf:

- **manuell** über den Abgleich-Knopf in der Web-Oberfläche (`POST /abgleich`), Fortschritt live per Server-Sent Events
  (`GET /ereignisse`);
- **automatisch** alle `DMARC_SYNC_INTERVAL_MINUTES` Minuten im
  Hintergrund (`internal/app/syncscheduler`, siehe
  [`deployment.md`](deployment.md));
- **per `docker exec`** (`dmarc-analyzer sync`) für Diagnose/Wartung.

Höchstens ein Lauf gleichzeitig — ein bereits laufender Sync macht einen
weiteren Startversuch zu einem No-Op, kein Fehler.

## Unterstützte Formate

DMARC-Aggregate-Reports (RUA, RFC 7489) kommen als E-Mail-Anhang in
verschiedenen Verpackungen an. Unterstützt werden `.xml`, `.xml.gz` und
`.zip` (auch mit **mehreren** enthaltenen XML-Dateien — jede wird als
eigener Report importiert, siehe `internal/infra/dmarcxml`s
`ParseAll`/`domainsync.MultiReportParser`). Ein Anhang, der von keinem
registrierten `sync.ReportParser` erkannt wird (z. B. der Text-Körper der
Nachricht selbst oder ein Firmenlogo im Anhang), wird stillschweigend
übersprungen — kein Fehler.

Unbekannte oder RFC-abweichende Enum-Werte im XML (Provider halten sich
nicht immer exakt an RFC 7489) werden nie verworfen, sondern auf einen
`Unknown`-Wert abgebildet — ein einzelnes abweichendes Feld darf den
Import des gesamten Reports nicht verhindern.

## Fehlerquarantäne

Kann eine Nachricht oder ein Anhang nicht verarbeitet werden (kaputtes
XML, unerwartetes Format), bricht das **nicht** den gesamten Sync-Lauf ab
— der Fehler landet in der Fehlerquarantäne (`failed_imports`-Tabelle,
`domainsync.FailedImportRepository`) inklusive der unverarbeiteten
Rohbytes, damit sich die Nachricht nach einer Parser-Korrektur erneut
einlesen lässt, ohne das Postfach erneut zu befragen.

## Datei-Import ohne IMAP

Reports lassen sich zusätzlich direkt über die Web-Oberfläche importieren (`/import`, Drag & Drop oder Dateiauswahl)
oder per
`docker exec dmarc-analyzer import <pfad>...` — ohne IMAP-Zugangsdaten,
für Offline-Betrieb oder die Migration von Altbeständen (`internal/app/importfiles`). Teilt sich MIME-Zerlegung und
Parser mit dem
IMAP-Sync, implementiert aber keine eigene `sync.MessageSource` — ein
lokaler Dateiimport hat keine IMAP-Zugangsdaten, durch dieses Interface zu
gehen wäre erzwungen statt natürlich.
