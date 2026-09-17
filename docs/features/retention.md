# Aufbewahrungsrichtlinie

DMARC-Aggregate-Reports werden nicht unbegrenzt aufgehoben — nach einer
konfigurierbaren Anzahl Monate werden sie automatisch gelöscht.

## Konfiguration

`DMARC_RETENTION_MONTHS` (Vorgabe: `24`, siehe
[`deployment.md`](deployment.md)). `0` bedeutet: unbegrenzte Aufbewahrung,
keine automatische Löschung. Eine Änderung braucht einen
Container-Neustart (12-factor: kein Laufzeit-Formular dafür, siehe
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`).

Die aktuell geltenden Werte (Aufbewahrungsdauer und Sync-Intervall) werden
auf der Status-Seite (`/einstellungen`) rein lesend angezeigt.

## Wie ein Report als "alt" gilt

Ein Report gilt als alt, sobald sein Berichtszeitraum vollständig vor dem
Stichtag `heute − DMARC_RETENTION_MONTHS Monate` endet — maßgeblich ist
das Ende des im Report selbst angegebenen Zeitraums (`date_end`), nicht
der Zeitpunkt des Imports.

## Wann die Regel angewendet wird

`internal/app/retentionjob.Runner` wendet die Regel automatisch im
Hintergrund an: einmal sofort beim Start des Containers (eine frisch
geänderte Aufbewahrungsdauer soll nicht erst nach einem vollen Tag
wirken) und danach alle 24 Stunden, für die gesamte Lebensdauer des
Containers.

## Implementierung

Löschen läuft über `report.Pruner.DeleteOlderThan` — ein schmaler,
eigener Port (`internal/domain/report/repository.go`), getrennt von
`report.Repository`, weil Löschen nach Alter eine reine
Wartungsoperation ist. Die SQLite-Implementierung
(`internal/infra/sqlite.ReportRepository.DeleteOlderThan`) löscht per
einem einzelnen `DELETE FROM reports WHERE date_end < ?` — die zugehörigen
Records, Auth-Ergebnisse, Reasons und der Rohbericht werden über
bestehende `ON DELETE CASCADE`-Fremdschlüssel automatisch mitentfernt.
