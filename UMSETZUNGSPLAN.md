# Umsetzungsplan — DMARC Analyzer

> Stand: 2026-09-16. Abgleich von `FEATURES.md` gegen den tatsächlichen
> Repository-Inhalt, danach der konkrete Arbeitsplan.
> `IMPLEMENTIERUNG.md` bleibt das Architekturdokument (das *Was* und *Warum*),
> dieses Dokument ist der Arbeitsplan (das *In welcher Reihenfolge*).

---

## 1. Ist-Stand

Das Repository enthält:

```
.gitignore          Go-Standard
FEATURES.md         Anforderungen (1 Zeile uncommitted geändert)
IMPLEMENTIERUNG.md  Architekturplan, 713 Zeilen
.idea/              IDE-Konfiguration
```

**Es existiert keine einzige Go-Datei.** Kein `go.mod`, kein `Taskfile.yml`,
kein `README.md`, kein `CHANGELOG.md`, keine Tests. Der Implementierungsgrad
gegenüber `FEATURES.md` liegt bei **0 %** — alles unten Beschriebene ist
Neubau, nichts ist Nacharbeit.

Werkzeuge lokal vorhanden und passend zum Plan: Go 1.27.1, Task 3.53.1,
golangci-lint 2.13.2.

---

## 2. Feature-Abgleich

| # | Anforderung aus `FEATURES.md` | Im Architekturplan | Im Code | Lücke |
| --- | --- | --- | --- | --- |
| F1 | DMARC-Daten analysieren und visualisieren | Abschnitt 6, 10.3 | — | AP 1–2, AP 6 |
| F2 | Oberfläche zum Erkunden und Verstehen der Daten | Abschnitt 10.1 | — | AP 5–6 |
| F3 | Verschiedene Datenformate, große Datenmengen | Abschnitt 7.1, 10.4 | — | **Formatmatrix fehlt** (siehe 3.2) |
| F4 | Filtern, Sortieren, Gruppieren | Abschnitt 10.1/10.3 | — | **Nur benannt, nicht spezifiziert** (siehe 3.3) |
| F5 | Diagramme und Grafiken | Abschnitt 10.3 | — | AP 6 |
| F6 | Daten aus einem Mailkonto holen | Abschnitt 7 | — | AP 3 |
| F7 | Zugangsdaten sicher speichern und nutzen | Abschnitt 9 | — | AP 3 |
| F8 | Nur neue Daten im Postfach entdecken | Abschnitt 7.1 | — | AP 3 |
| F9 | Zusätzliche Funktionen vorschlagen | Abschnitt 11 | — | erledigt, Auswahl offen |
| N1 | Go-Binary mit Fyne | Abschnitt 3 | — | AP 0 |
| N2 | Credentials sicher gespeichert | Abschnitt 9 | — | AP 3 |
| N3 | Taskfile mit den üblichen Tasks | Abschnitt 13 | — | AP 0 |
| N4 | Tests | Abschnitt 12 | — | in jedem AP |
| N5 | README.md | Abschnitt 14 | — | AP 0 (Rohfassung), AP 7 (final) |
| N6 | CHANGELOG.md | Abschnitt 14 | — | AP 0, danach je AP gepflegt |
| N7–N10 | SOLID, DDD, Clean Architecture, Clean Code | Abschnitt 4.2 | — | Struktur ab AP 0, Prüfung per Lint |
| N11 | Fyne Best Practices | Abschnitt 10.4 | — | AP 5 |
| **N12** | **Benutzerfreundliche Oberfläche für einfache Navigation und Bedienung** | **nur implizit** | — | **neu hinzugekommen, siehe 3.1** |

---

## 3. Was im Architekturplan nachgeschärft werden muss

### 3.1 N12 — Benutzerfreundlichkeit ist neu und bisher nur implizit abgedeckt

Diese Zeile kam nach dem Schreiben von `IMPLEMENTIERUNG.md` in `FEATURES.md`
dazu und ist dort nicht als eigenständige Anforderung behandelt. Abschnitt 10
beschreibt *welche Ansichten* es gibt, aber nicht, woran sich „benutzerfreundlich"
messen lässt. Verbindliche Kriterien für AP 5–7:

* **Ersteinrichtung geführt** — beim ersten Start ein Assistent in drei Schritten
  (Konto → Verbindung testen → erster Sync). Kein leeres Fenster ohne Hinweis.
* **Leerzustände mit Handlungsaufforderung** — jede Ansicht ohne Daten erklärt,
  *warum* sie leer ist und welcher Knopf weiterhilft.
* **Fachbegriffe erklärt** — DMARC-Vokabular (Disposition, Alignment, Policy)
  bekommt Tooltips und ein Glossar. Zielgruppe versteht Mail, nicht RFC 7489.
* **Fehlermeldungen in Klartext** — „Anmeldung fehlgeschlagen — bei aktivierter
  Zwei-Faktor-Authentifizierung wird ein App-Passwort benötigt." statt
  `imap: NO [AUTHENTICATIONFAILED]`. Technischer Originaltext aufklappbar.
* **Tastaturbedienung** — vollständige Tab-Reihenfolge, `Cmd/Ctrl+R` Sync,
  `Cmd/Ctrl+F` Filter, `Esc` schließt Dialoge.
* **Keine blockierende UI** — jede Aktion über 300 ms zeigt Fortschritt und
  lässt sich abbrechen.
* **Barrierefreiheit** — Pass/Fail nie nur über Farbe, Mindestkontrast 4,5:1,
  Schriftgröße folgt der Systemeinstellung.

### 3.2 F3 — „Various data formats" braucht eine verbindliche Liste

Der Plan nennt `.gz`, `.zip` und blankes `.xml`. Für AP 1 festgelegt:

| Format | Umgang |
| --- | --- |
| `report.xml` | direkt parsen |
| `report.xml.gz` | `compress/gzip`, Größenlimit 100 MB |
| `report.zip` | `archive/zip`, **alle** enthaltenen XML-Dateien, Limit pro Eintrag und Summe |
| `.eml`-Datei | MIME zerlegen wie eine IMAP-Nachricht (Datei-Import, Vorschlag 11.1) |
| base64-Inline-Body ohne Anhang | erkennen und dekodieren — kommt bei einzelnen Providern vor |
| unbekannt | Quarantäne in `failed_imports`, Lauf läuft weiter |

### 3.3 F4 — Filtern/Sortieren/Gruppieren gehört in SQL, nicht in die UI

Damit „große Datenmengen effizient" (F3) nicht zur Lüge wird: `ReportQuery`
bekommt in AP 2 Filterfelder (Zeitraum, Domain, Org, Quell-IP, Disposition,
Pass/Fail), Sortierschlüssel mit Richtung, Gruppierungsdimension und
Keyset-Pagination. Die UI-Tabelle in AP 5 lädt ausschließlich sichtbare Seiten
nach. Kein `SELECT *` ins RAM, keine Sortierung in Go.

### 3.4 Modulpfad und Keyring-Service sind falsch vorgeschlagen

`IMPLEMENTIERUNG.md` schlägt `github.com/Freie-Schule/dmarc-analyzer` und den
Keyring-Service `de.freie-schule.dmarc-analyzer` vor. Das Repository liegt unter
`pmoscode/dmarc-analyzer`. **Festlegung:**

* Modulpfad: `github.com/pmoscode/dmarc-analyzer`
* Keyring-Service: `de.pmoscode.dmarc-analyzer`

Der Keyring-Service lässt sich später nur mit Migrationsaufwand ändern — das
gehört vor die erste Zeile Code entschieden, nicht danach.

### 3.5 Offene Fragen: Vorschlagswerte gelten, bis widersprochen wird

O-2 bis O-8 aus `IMPLEMENTIERUNG.md` werden mit den dort vorgeschlagenen
Antworten umgesetzt (englische Bezeichner, MIT-Lizenz, `BODY.PEEK` ohne
Markierung, 24 Monate Aufbewahrung, CLI-Modus ja, alle drei Plattformen bauen).
Nur O-5 bleibt echt offen: **echte Beispiel-Reports aus dem Postfach wären für
AP 1 wertvoll.** Ohne sie werden synthetische Fixtures aus den RFC-Beispielen
erzeugt — das deckt die Provider-Eigenheiten schlechter ab.

---

## 4. Arbeitspakete

Jedes AP endet mit grünem `task check` und einem CHANGELOG-Eintrag.
Die APs 0–4 sind vollständig ohne UI testbar.

### AP 0 — Grundgerüst

**Ziel:** `task check` läuft grün auf einem leeren, aber vollständig
konfigurierten Projekt.

- [x] `go mod init github.com/pmoscode/dmarc-analyzer`
- [x] Verzeichnisbaum nach `IMPLEMENTIERUNG.md` Abschnitt 5 anlegen
- [x] Abhängigkeiten recherchiert und **auf konkrete Versionen gepinnt** (Fyne, go-imap/v2,
      go-message, modernc.org/sqlite, go-keyring, go-chart/v2, testify) — Details und eine
      Abweichung von der ursprünglichen Idee in `docs/DEPENDENCIES.md`: nur `testify` steht
      bereits in `go.mod`, weil es ab AP 0 tatsächlich in Tests importiert wird. Die übrigen
      fünf sind dort mit Zielversion für ihre jeweilige Phase dokumentiert statt ungenutzt in
      `go.mod` erzwungen — sonst entfernt sie der nächste `task tidy` wieder, und `go.mod`
      würde über den Ist-Zustand lügen.
- [x] `Taskfile.yml` mit den 16 Tasks aus Abschnitt 13
- [x] `.golangci.yml`: govet, staticcheck, errcheck, revive, gosec, ineffassign, gocritic —
      **`misspell` bewusst weggelassen**: die deutschen Kommentare (Projektkonvention, siehe
      1.3) erzeugen mit einem englischen Wörterbuch ständig Fehlalarme
      (`Konfiguration` → `Configuration`), begründet in `.golangci.yml`.
- [x] `.github/workflows/ci.yml`: `task check` + `task build` auf ubuntu/macos/windows,
      zusätzlich Prüfung auf gofmt-/goimports-Drift
- [x] `internal/platform/logging` (slog, Textformat lokal, JSON per Option)
- [x] `internal/platform/paths` (DB-, Log-, Konfigpfad plattformkonform, mit Tests)
- [x] `cmd/dmarc-analyzer/main.go` — startet, loggt Version, beendet sich, mit Tests
- [x] `README.md` und `CHANGELOG.md` als Rohfassung, `LICENSE` (MIT)

**Fertig wenn:** `task build` erzeugt eine lauffähige Binärdatei, `task check`
ist grün, CI ist auf allen drei Plattformen grün. — Lokal erreicht (`task check`,
`task build` beide grün, Coverage `platform/*` 83 %). CI-Grün auf allen drei
Plattformen lässt sich erst nach dem ersten Push/PR verifizieren.

### AP 1 — Domäne und Parser

**Ziel:** Ein beliebiger Report wird korrekt zu Domänenobjekten.

- [ ] `internal/domain/report`: Entities, Value Objects, Konstruktoren mit
      Invarianten (Abschnitt 6.1/6.2)
- [ ] Unbekannte Enum-Werte → `Unknown`, nie verwerfen
- [ ] `internal/domain/sync/ports.go`: `MessageSource`, `ReportParser`
- [ ] `internal/infra/dmarcxml`: XML-Parser, tolerant gegenüber
      Provider-Abweichungen
- [ ] Entpacker für die Formatmatrix aus 3.2, inkl. Größenlimit gegen Zip-Bomben
- [ ] Fixtures in `testdata/reports/` (anonymisiert: `example.com`, RFC-5737-IPs)
- [ ] Golden-File-Tests je Provider, Fuzz-Test auf dem XML-Parser
- [ ] XXE-Test, der festschreibt, dass externe Entities nicht aufgelöst werden

**Fertig wenn:** Abdeckung `domain` ≥ 90 %, `dmarcxml` ≥ 85 %, Fuzz läuft 60 s
ohne Fund.

### AP 2 — Persistenz

**Ziel:** Reports werden gespeichert, dedupliziert und performant abgefragt.

- [ ] SQLite-Verbindung mit PRAGMAs (WAL, foreign_keys, busy_timeout, synchronous)
- [ ] Eigener Migrator + eingebettete `migrations/*.sql` (Schema Abschnitt 8.1)
- [ ] `reportrepo.go` mit Batch-Insert in einer Transaktion
- [ ] Deduplizierung über UNIQUE `(org_name, report_id, date_begin)`
- [ ] `ReportQuery` vollständig nach 3.3 (Filter, Sortierung, Gruppierung, Keyset)
- [ ] Aggregationen für die Kennzahlen aus Abschnitt 10.2 **in SQL**
- [ ] `raw_reports` und `failed_imports`
- [ ] Integrationstests gegen eine temporäre Datei-DB, Migration vorwärts und
      wiederholt
- [ ] Benchmark: 100.000 Records importieren und abfragen

**Fertig wenn:** Doppelimport derselben Mail erzeugt genau einen Datensatz;
Abfrage über 100.000 Records unter 100 ms.

### AP 3 — Mail und Sicherheit

**Ziel:** Aus einem echten Postfach wird nur Neues geholt.

- [ ] IMAP-Adapter: TLS erzwungen, `BODY.PEEK`, UID-basiert, streamender Iterator
- [ ] `UIDVALIDITY`-Wechsel → vollständiger Rescan, Duplikate fängt der Index
- [ ] `SyncState` nach jeder Nachricht fortschreiben
- [ ] Backoff bei temporären Fehlern (3 Versuche)
- [ ] Keyring-Adapter, Service `de.pmoscode.dmarc-analyzer`
- [ ] Typ `Secret` mit maskierendem `String()`/`MarshalJSON()` + Test, der
      beweist, dass `%v` und JSON nichts preisgeben
- [ ] Linux-Fallback: AES-256-GCM-Dateispeicher, Schlüssel via scrypt
- [ ] Kontolöschung entfernt Metadaten **und** Keyring-Eintrag
- [ ] Tests gegen einen In-Process-IMAP-Server, besonders UID-Logik

**Fertig wenn:** Zweiter Sync-Lauf direkt nach dem ersten holt null Nachrichten;
kein Testlauf enthält ein Passwort in Logs oder Fehlertexten.

### AP 4 — Anwendungsschicht

**Ziel:** Der komplette Ablauf ist ohne UI nutzbar.

- [ ] Use Cases `syncreports`, `queryreports`, `statistics`, `manageaccount`,
      `exportdata`
- [ ] Pipeline Abholen/Parsen nebenläufig, Schreiben seriell, `-race` sauber
- [ ] Fortschritts- und Ergebnisberichte (neu / übersprungen / fehlerhaft)
- [ ] Datei-Import als zweite `MessageSource` (Vorschlag 11.1)
- [ ] CLI: `dmarc-analyzer sync --headless`, `import <pfad>`, `stats`
- [ ] Alle Use Cases gegen handgeschriebene Fakes getestet, Abdeckung ≥ 85 %
- [ ] Abbruch per `context.Cancel` hinterlässt konsistenten Zustand (Test)

**Fertig wenn:** Ein vollständiger Import aus einem Postfach läuft über die CLI
durch, inklusive Kennzahlenausgabe.

### AP 5 — UI-Grundgerüst

**Ziel:** Bedienbares Programm für den Kern-Use-Case.

- [ ] Hauptfenster, seitliche Navigation, eigenes Theme (hell und dunkel)
- [ ] `internal/ui/i18n` — **alle** Texte zentral, kein String im Widget-Code
- [ ] Einstellungen: Konto anlegen, Verbindung testen mit klarer Rückmeldung
- [ ] Ersteinrichtungs-Assistent nach 3.1
- [ ] Berichtstabelle mit Lazy-Datenquelle über `ReportQuery`
- [ ] Detailansicht eines Reports
- [ ] Sync-Knopf mit Fortschritt und Abbruch, I/O nie im UI-Thread,
      Rückweg über `fyne.Do()`
- [ ] Leerzustände und Klartext-Fehlermeldungen nach 3.1
- [ ] Tests mit `fyne.io/fyne/v2/test`: Aufbau, Navigation, Formularvalidierung

**Fertig wenn:** Ein Nutzer richtet ohne Dokumentation ein Konto ein und sieht
seine Reports.

### AP 6 — Auswertung und Visualisierung

**Ziel:** Die in `FEATURES.md` geforderte Visualisierung steht.

- [ ] Dashboard mit Kennzahlen-Kacheln inkl. Trend zur Vorperiode
- [ ] `ChartRenderer`-Port + go-chart-Adapter
- [ ] Zeitreihe, Top-10-Balken, Disposition-Donut, Heatmap Quelle × Tag
- [ ] Sendequellen-Ansicht, aggregiert nach Quell-IP
- [ ] rDNS/PTR-Auflösung mit Cache (Vorschlag 11.2)
- [ ] Erkennung bekannter Dienste über PTR und IP-Bereiche (Vorschlag 11.3)
- [ ] Filter- und Gruppierungsleiste, wirkt auf alle Ansichten
- [ ] Glossar und Tooltips für DMARC-Begriffe
- [ ] Export: gefilterte Ansicht als CSV, Diagramm als PNG (Vorschlag 11.4)

**Fertig wenn:** Aus dem Dashboard ist ohne Zwischenschritt erkennbar, welche
Sendequelle fehlschlägt und wie viele Nachrichten betroffen sind.

### AP 7 — Feinschliff und Release 1.0.0

- [ ] Aufbewahrungsrichtlinie, Standard 24 Monate (Vorschlag 11.11)
- [ ] Hintergrund-Sync nach Zeitplan + Desktop-Benachrichtigung
- [ ] Backup/Restore per `VACUUM INTO` (Vorschlag 11.12)
- [ ] Barrierefreiheits-Durchgang nach den Kriterien aus 3.1
- [ ] Lasttest mit mehreren Jahren Reportdaten
- [ ] Packaging macOS/Windows/Linux, `task release` mit Checksummen
- [ ] README final mit Screenshots und Datenschutzhinweis, ADRs in `docs/`
- [ ] CHANGELOG-Eintrag `1.0.0`, Tag setzen

---

## 5. Reihenfolge und Abhängigkeiten

```
AP 0 ──▶ AP 1 ──▶ AP 2 ──▶ AP 3 ──▶ AP 4 ──▶ AP 5 ──▶ AP 6 ──▶ AP 7
                    │                          │
                    └──── AP 4 braucht 2 + 3 ──┘
```

AP 1 und AP 2 lassen sich teilweise parallel bearbeiten (Parser gegen Fixtures,
Schema gegen Handdaten), AP 3 braucht beide. Vor AP 5 sollte AP 4 vollständig
getestet sein — sonst werden UI-Fehler und Logikfehler ununterscheidbar.

---

## 6. Vor dem ersten Commit zu entscheiden

| Nr. | Frage | Vorbelegung, wenn keine Antwort kommt | Status |
| --- | --- | --- | --- |
| E-1 | Modulpfad `github.com/pmoscode/dmarc-analyzer`? | ja | **umgesetzt** (AP 0) |
| E-2 | Keyring-Service `de.pmoscode.dmarc-analyzer`? | ja | **umgesetzt** (AP 0, in README dokumentiert; Adapter folgt AP 3) |
| E-3 | Lizenz MIT? | ja | **umgesetzt** (AP 0, `LICENSE`) |
| E-4 | Echte Beispiel-Reports für die Fixtures verfügbar? | nein → synthetische aus RFC-Beispielen | offen — vor AP 1 klären |
| E-5 | Umfang v1: APs 0–7 wie oben, oder Schnitt nach AP 5? | voller Umfang bis AP 7 | offen (Vorbelegung gilt vorerst) |

E-1 bis E-3 sind nachträglich nur mit Migrationsaufwand änderbar — sie sind mit
AP 0 jetzt fixiert.
