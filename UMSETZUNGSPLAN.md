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

- [x] `internal/domain/report`: Entities, Value Objects, Konstruktoren mit
      Invarianten (Abschnitt 6.1/6.2) — inkl. `Repository`-Port (Abschnitt 6.4),
      vorgezogen aus AP 2, weil die Schnittstelle neben dem Aggregate hingehört
      (Implementierung bleibt AP 2)
- [x] Unbekannte Enum-Werte → `Unknown`, nie verwerfen — für Disposition, Policy,
      AlignmentMode und AuthResultValue, mit Golden-File-Test abgesichert
- [x] `internal/domain/sync`: `State`, `ReportParser` — **`MessageSource` bewusst
      noch nicht definiert**, siehe Kommentar in `ports.go`: der Port hängt von
      `account.MailAccount` ab, das erst in AP 3 entsteht. Ihn jetzt mit einem
      Platzhalter-Kontotyp einzuführen hieße, eine Abhängigkeit vorzutäuschen,
      die es noch nicht gibt. Aus demselben Grund heißt der Typ `sync.State`
      statt `sync.SyncState` (vermeidet den Stutter `sync.SyncState`) — dieselbe
      Konsequenz wurde in `report.Key`/`Query`/`Page`/`Repository` gezogen
      (statt `ReportKey` etc.), `ReportID` blieb Ausnahme (Feldkollision mit
      `AggregateReport.ID`, siehe Kommentar im Code).
- [x] `internal/infra/dmarcxml`: XML-Parser, tolerant gegenüber
      Provider-Abweichungen (Groß-/Kleinschreibung, fehlendes `pct`, fehlendes
      `adkim`/`aspf` mit RFC-Default `r`, unbekannte Enum-Werte)
- [x] Entpacker für die Formatmatrix aus 3.2 (`.xml`, `.xml.gz`, `.zip` mit
      mehreren XML-Dateien), Größenlimit 100 MB gegen Zip-/Gzip-Bomben
- [x] Fixtures in `testdata/reports/` — **synthetisch aus RFC-7489-Beispielen**
      (E-4 blieb unbeantwortet, Fallback aus Abschnitt 6 angewendet), fünf
      Provider-Stile (RFC-Beispiel, Google, Microsoft, Multi-Report-Zip, drei
      Sonderfälle), `example.com`/RFC-5737-Adressen
- [x] Golden-File-Tests je Provider-Stil, Fuzz-Test (`FuzzParse`, 30 s / 2,26 Mio.
      Durchläufe ohne Fund lokal verifiziert)
- [x] XXE-Test — mit einer Korrektur gegenüber der ursprünglichen Annahme:
      `encoding/xml` löst die externe Entity nicht still auf, sondern lehnt das
      ganze Dokument ab. Der Test prüft jetzt genau das (Fehler, kein
      Erfolg mit ignorierter Entity).

**Fertig wenn:** Abdeckung `domain` ≥ 90 %, `dmarcxml` ≥ 85 %, Fuzz läuft 60 s
ohne Fund. — Erreicht: `domain/report` 100 %, `infra/dmarcxml` 90,7 %, Fuzz
30 s ohne Fund (lokal; 60 s sind in CI oder vor dem Release nachzuholen).
Die beiden Zip-/Gzip-Bomben-Tests sind mit `testing.Short()` markiert, damit
`task test:unit` schnell bleibt (~1,5 s statt ~7 s).

### AP 2 — Persistenz

**Ziel:** Reports werden gespeichert, dedupliziert und performant abgefragt.

- [x] SQLite-Verbindung mit PRAGMAs (WAL, foreign_keys, busy_timeout, synchronous)
      — als DSN-Parameter beim Öffnen, nicht als separate PRAGMA-Statements
      (`_journal_mode`/`_foreign_keys`/`_busy_timeout`/`_synchronous`,
      modernc.org/sqlite-eigene Kurzform)
- [x] Eigener Migrator (~150 Zeilen) + eingebettete `migrations/*.sql`
      (Schema Abschnitt 8.1, erweitert um `report_errors` sowie
      `mailbox`/`filename`/`org_extra_contact_info` auf `reports` — die
      Abschnitt-8.1-Tabelle war als „Auszug" markiert, das war die Lücke
      zum vollständigen Domänenmodell aus AP 1)
- [x] `reportrepo.go` mit Batch-Insert (vorbereitete Statements je Tabelle,
      wiederverwendet über alle Records eines Reports) in einer Transaktion
- [x] Deduplizierung über UNIQUE `(org_name, report_id, date_begin)`,
      als `ErrDuplicateReport` über `sqlite.Error.Code()` erkannt (nicht
      String-Matching auf die Fehlermeldung)
- [x] `Query` vollständig nach 3.3: Filter (Zeitraum-Überlappung, Domain,
      Org, Quell-IP, Disposition über `EXISTS`-Subqueries), Sortierung,
      Keyset-Pagination über SQLite-Row-Value-Vergleiche. **Präzisierung zu
      `GroupBy`:** wirkt als zusätzlicher primärer Sortierschlüssel (gleiche
      Gruppe steht zusammen), nicht als SQL-`GROUP BY` mit Aggregation —
      `Query` liefert laut Portvertrag weiterhin einzelne `AggregateReport`,
      keine Kennzahlen. `GroupBySourceIP` lehnt der Adapter mit Fehler ab
      (Quell-IP ist eine Record-, keine Report-Eigenschaft). Siehe
      Kommentare in `domain/report/repository.go`.
- [ ] Aggregationen für die Kennzahlen aus Abschnitt 10.2 **in SQL** —
      **bewusst nicht in AP 2.** Das sind anwendungsseitige Fragen
      (Pass-Rate nach Alignment-Definition, Trend zur Vorperiode), keine
      reine Persistenzaufgabe — gehören zum Statistics-Use-Case in AP 4,
      der dafür eigene, zugeschnittene Repository-Methoden bekommt statt
      `Query`/`GroupBy` zu überladen.
- [x] `raw_reports` und `failed_imports` — **nur das Schema** (Tabellen +
      Indizes existieren). Tatsächliches Schreiben (Rohdaten-Archiv,
      Fehlerquarantäne beim Sync) ist Teil des SyncReports-Use-Case in
      AP 3/4, nicht der reinen Persistenzschicht.
- [x] Integrationstests gegen eine temporäre Datei-DB (nie `:memory:`),
      Migration wird in `TestMigrate_IsIdempotent` zweimal angewendet
- [x] Benchmark: 100.000 Records importieren und abfragen — **als
      regulärer, mit `testing.Short()` übersprungener Test** statt
      `go test -bench`: das Szenario ist ein einmaliger Ablauf (Datenmenge
      aufbauen, eine realistische Abfrage messen), keine Mikro-Benchmark-
      Schleife. Ergebnisse siehe unten.

**Ein echter Performance-Bug unterwegs gefunden und behoben:** Die erste
Fassung der Batch-Ladefunktionen für Records/Reasons/Auth-Ergebnisse baute
eine `IN (?, ?, ..., ?)`-Klausel mit einem Platzhalter je Record — bei
10.000 Records dauerte allein das Vorbereiten dieser Statements so lange,
dass `FindByID` über 3 Sekunden brauchte. Zusätzlich fehlten Indizes auf
`record_reasons.record_id`, `auth_results_dkim.record_id`,
`auth_results_spf.record_id` und `report_errors.report_id` (das Schema
zieht schon vor dem ersten Release, kein `0002`-Migrationsumweg). Beides
behoben: Indizes ergänzt, die drei Ladefunktionen von IN-Klausel auf
`JOIN records ON ... WHERE records.report_id = ?` umgestellt.

**Fertig wenn:** Doppelimport derselben Mail erzeugt genau einen Datensatz —
erreicht (`TestSave_DuplicateKey_ReturnsErrDuplicateReport`). Abfrage über
100.000 Records unter 100 ms — erreicht für den eigentlichen Zielpfad, die
Berichtstabelle: lokal gemessen **~7-8 ms** für eine sortierte, gefilterte
Seite über 1.001 Reports mit 110.000 Records insgesamt. Das Laden eines
einzelnen, unrealistisch großen Reports mit 10.000 Records (`FindByID`,
Detailansicht) liegt bei **~460 ms** — spürbar über 100 ms, aber ein
Randfall (reale DMARC-Reports haben selten mehr als niedrige Hunderte
Records) und für eine einmalige Detailansicht akzeptabel; als möglicher
Optimierungspunkt für später vorgemerkt, kein Blocker für AP 2.

### AP 3 — Mail und Sicherheit

**Ziel:** Aus einem echten Postfach wird nur Neues geholt.

- [x] IMAP-Adapter (`internal/infra/imap.Adapter`): TLS erzwungen sofern
      `MailAccount.UseTLS` (context-fähiger Dial über `tls.Dialer`/
      `net.Dialer`, nie `InsecureSkipVerify`), `BODY.PEEK[]` **und**
      `EXAMINE` statt `SELECT` (read-only, doppelt abgesichert), UID-basiert,
      streamender `iter.Seq2`-Iterator über `FetchMessageData.Collect()` je
      Nachricht (nicht die ganze Ergebnismenge auf einmal)
- [x] `UIDVALIDITY`-Wechsel → vollständiger Rescan (`startUID = 1`,
      `baseline.LastUID = 0`), Duplikate fängt der UNIQUE-Index aus AP 2 ab
      — mit In-Process-Server verifiziert (`TestFetchNew_
      UIDValidityChange_TriggersFullRescan`)
- [x] `SyncState` (`sync.State`) nach jeder Nachricht fortschreiben —
      **Präzisierung:** `FetchNew` selbst schreibt nichts fort, sondern
      liefert einen Baseline-State (v. a. die aktuelle `UIDValidity`) plus
      pro Nachricht deren UID; der Aufrufer (SyncReports-Use-Case, AP 4)
      schreibt `LastUID` nach jeder erfolgreich verarbeiteten Nachricht
      fort. Siehe Kommentar an `sync.MessageSource` in `ports.go`.
- [x] Backoff bei temporären Fehlern (3 Versuche, exponentiell) — **nur für
      den Verbindungsaufbau**, nicht für Login: ein falsches Passwort wird
      durch Wiederholen nicht richtig und wiederholte Fehlversuche können
      Konten beim Provider sperren. Mit einem echten transienten Fehler
      verifiziert (`TestConnect_RetriesTransientDialFailure`: Server startet
      erst während der Backoff-Wartezeit zwischen Versuch 1 und 2).
- [x] Keyring-Adapter (`internal/infra/keyring.OSStore`), Service
      `de.pmoscode.dmarc-analyzer`
- [x] Typ `Secret` (`internal/domain/account`) mit maskierendem
      `String()`/`MarshalJSON()` + Tests. **Ein echtes Leck dabei gefunden
      und behoben:** `%#v` benutzt `fmt.GoStringer`, nicht `fmt.Stringer —
      ohne eigene `GoString()`-Methode hätte `%#v` die rohen Secret-Bytes
      hex-kodiert gezeigt, obwohl `%v`/`%+v` bereits korrekt maskierten. Der
      erste Testentwurf hätte das nicht gemerkt (ASCII-Substring-Suche
      erkennt Hex-Kodierung nicht) — Test entsprechend verschärft. Siehe
      AGENTS.md, Abschnitt „Maskierte Typen".
- [x] Linux-Fallback (`internal/infra/keyring.FileStore`): AES-256-GCM
      (Standardbibliothek `crypto/aes`/`crypto/cipher`), Schlüssel via
      `scrypt` (neu gepinnt: `golang.org/x/crypto`, siehe
      `docs/DEPENDENCIES.md`) aus Master-Passphrase + persistiertem Salt.
      `keyring.IsAvailable()` prüft den OS-Schlüsselbund per Kanarienwert
      statt aus einer bestimmten Fehlerklasse zu raten — die Entscheidung
      *welcher* Store verwendet wird, trifft die Composition Root (AP 4/5),
      nicht der Adapter selbst.
- [~] Kontolöschung entfernt Metadaten **und** Keyring-Eintrag — **nur die
      Keyring-Hälfte ist AP 3**: `CredentialStore.Delete` ist implementiert
      und getestet (idempotent). Die Metadaten-Hälfte braucht
      `account.Repository` gegen SQLite (`accountrepo.go`), das bewusst wie
      in AP 1/2 für `MessageSource` begründet erst in AP 4 entsteht, wenn
      die Kontoverwaltung (`internal/app/manageaccount`) beide Hälften
      zusammenführt. Der Port `account.Repository` ist bereits definiert.
- [x] Tests gegen einen In-Process-IMAP-Server (`imapmemserver`, go-imaps
      eigene Testserver-Komponente), 11 Tests inkl. UID-Logik,
      Kontext-Abbruch (vor und während der Iteration), PEEK/kein-\Seen-Flag.
      **Ein Bug in der Beta-Bibliothek gefunden:**
      `imapmemserver.User.Append` mit `nil`-Options dereferenziert
      ungeprüft und panickt — mit `&imap.AppendOptions{}` umgangen,
      dokumentiert im Test.

**Fertig wenn:** Zweiter Sync-Lauf direkt nach dem ersten holt null
Nachrichten — erreicht, `TestFetchNew_SecondSync_ReturnsNoNewMessages` bildet
genau das nach. Kein Testlauf enthält ein Passwort in Logs oder
Fehlertexten — die Testserver-Logs wurden geprüft, `Secret` verhindert es
strukturell; einzige bekannte Grenze: die an `imapclient.Login` als
`string` übergebene Kopie des Passworts kann nicht mehr aktiv überschrieben
werden (Go-Strings sind unveränderlich), siehe Kommentar an `Secret.Zero()`.

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
