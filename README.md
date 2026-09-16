# DMARC Analyzer

Ein eigenständiges Desktop-Programm für macOS, Windows und Linux, das
DMARC-Aggregate-Reports (RUA) aus einem IMAP-Postfach abholt, lokal
auswertet und visualisiert.

> **Status:** Frühe Entwicklungsphase (AP 0 — Grundgerüst). Es gibt noch
> keine funktionsfähige Anwendung. Details zu Architektur und Zielbild
> stehen in [`IMPLEMENTIERUNG.md`](IMPLEMENTIERUNG.md), der aktuelle
> Arbeitsplan in [`UMSETZUNGSPLAN.md`](UMSETZUNGSPLAN.md).

## Was das Programm tun wird

- Verbindet sich mit einem konfigurierten IMAP-Postfach und holt
  **inkrementell nur neue** DMARC-Aggregate-Reports ab (`.xml`, `.xml.gz`,
  `.zip`).
- Speichert die ausgewerteten Daten lokal in SQLite — keine Cloud-Anbindung.
- Zeigt Kennzahlen (Pass-Rate, Alignment, Volumen nach Disposition),
  Zeitreihen und eine nach Sendequelle aggregierte Ansicht.
- Filtert, sortiert und gruppiert nach Zeitraum, Domain, Absender-Org und
  Quell-IP.
- Speichert Zugangsdaten ausschließlich im Schlüsselbund des
  Betriebssystems, niemals im Klartext in der Datenbank.

## Installation

Vorgefertigte Binärdateien folgen mit dem ersten Release. Bis dahin: siehe
Abschnitt „Entwicklungs-Setup".

## Einrichtung des IMAP-Kontos

Wird mit AP 5 (UI-Grundgerüst) ergänzt, sobald die Kontoverwaltung
existiert. Vorgesehen: Host, Port, Benutzername, App-Passwort (empfohlen
bei aktivierter Zwei-Faktor-Authentifizierung), Postfachname.

## Kennzahlen

Wird mit AP 6 (Auswertung) ergänzt. Siehe
[`IMPLEMENTIERUNG.md`, Abschnitt 10.2](IMPLEMENTIERUNG.md#102-kennzahlen-auf-der-übersicht)
für die geplante Definition.

## Speicherorte

| Inhalt | Ort |
| --- | --- |
| Datenbank (`dmarc.db`) | `os.UserConfigDir()/dmarc-analyzer/` |
| Konfiguration | `os.UserConfigDir()/dmarc-analyzer/` |
| Logs | macOS: `~/Library/Logs/dmarc-analyzer/`, sonst wie Konfiguration |
| Zugangsdaten | OS-Schlüsselbund, Service `de.pmoscode.dmarc-analyzer` |

## Datenschutz

DMARC-Aggregate-Reports enthalten IP-Adressen von Mailversendern. Die
Verarbeitung erfolgt ausschließlich lokal, ohne Cloud-Übertragung. Eine
konfigurierbare Aufbewahrungsrichtlinie ist für AP 7 vorgesehen.

## Entwicklungs-Setup

Voraussetzungen: Go 1.27+, [Task](https://taskfile.dev/).

```sh
task setup   # Werkzeuge installieren, Abhängigkeiten laden
task check   # fmt + lint + test — vor jedem Commit
task run     # Programm im Entwicklungsmodus starten
```

Alle verfügbaren Tasks: `task --list`. Details zu Architektur, Teststrategie
und Phasenplan stehen in [`IMPLEMENTIERUNG.md`](IMPLEMENTIERUNG.md).

## Lizenz

[MIT](LICENSE)
