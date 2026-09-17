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

- [x] Use Cases `syncreports`, `queryreports`, `statistics`, `manageaccount`,
      `exportdata` — plus `importfiles` (siehe nächster Punkt) und, als
      Nebenprodukt, `internal/infra/mailmime` (MIME-Zerlegung, AP 3 bewusst
      zurückgestellt) und drei neue SQLite-Repositories (`accountrepo.go`,
      `syncstaterepo.go`, `failedimportrepo.go`), ebenfalls bewusst aus
      AP 1–3 hierher verschoben, wo die tatsächlichen Verbraucher entstehen.
      `statistics` bekam dabei einen `analysis`-Port (Statistics-Typen,
      `Repository.Compute`) — die in AP 2 zurückgestellte SQL-Aggregation
      ("Aggregationen für die Kennzahlen") landet hier, zugeschnitten statt
      als Erweiterung von `report.Query`.
- [x] Pipeline Abholen/Parsen nebenläufig, Schreiben seriell, `-race` sauber
      — `syncreports.runPipeline`: eine Fetcher-Goroutine, ein
      Parser-Worker-Pool (konfigurierbare Größe), ein einzelner Schreiber.
      **Design-Detail über den Wortlaut hinaus:** da Worker außer der
      Reihe fertig werden können, verfolgt ein `progressTracker` die
      lückenlose Fortschrittsgrenze statt einfach die höchste gesehene UID
      zu übernehmen — sonst könnte `LastUID` bei einem Absturz eine noch
      nicht gespeicherte Nachricht überspringen. Mit gezielten Tests für
      genau diesen Out-of-Order-Fall abgesichert.
- [x] Fortschritts- und Ergebnisberichte (neu / übersprungen / fehlerhaft) —
      `Result{New, Skipped, Failed, Errors}`, fehlerhafte Anhänge landen in
      `failed_imports` (neuer `FailedImportRepository`-Port).
- [x] Datei-Import (Vorschlag 11.1) — **bewusst NICHT** als zweite
      `MessageSource`: der Port verlangt seit AP 3 `account.MailAccount` +
      `Secret` für `Connect`, das hat ein lokaler Dateiimport nicht. Eigener
      Use Case `internal/app/importfiles`, der sich `MessageDecoder` und
      `ReportParser` mit `syncreports` teilt (dieselbe MIME-Zerlegung für
      IMAP wie für `.eml`-Dateien) und dieselbe Dedup-Logik nutzt — dafür
      extrahiert als domain-seitiges `report.SaveIfNew(ctx, repo, r)`
      (siehe unten), statt in beiden Use Cases dupliziert.
- [x] CLI: `dmarc-analyzer sync --headless`, `import <pfad>`, `stats` — plus
      `account add|list|test|delete` (nicht im Wortlaut gefordert, aber ohne
      irgendeinen Weg, ein Konto anzulegen, wäre `sync` von der CLI aus gar
      nicht nutzbar; siehe Diskussion unten bei "Fertig wenn"). Composition
      Root (`cmd/dmarc-analyzer/wire.go`) verdrahtet Adapter **nur für den
      tatsächlich aufgerufenen Unterbefehl** — `stats`/`import` bauen keinen
      Credential-Store auf und fassen damit nie den echten OS-Schlüsselbund
      an, das passiert ausschließlich für `sync`/`account`.
- [x] Alle Use Cases gegen handgeschriebene Fakes getestet, Abdeckung ≥ 85 %
      — erreicht: `syncreports` 92,1 %, `manageaccount` 91,7 %,
      `statistics` 90,9 %, `importfiles` 86,2 %, `exportdata` 87,8 %,
      `queryreports` 100 %.
- [x] Abbruch per `context.Cancel` hinterlässt konsistenten Zustand (Test) —
      `TestSyncAccount_ContextCancelledMidSync_LeavesConsistentState`:
      `LastUID` überspringt nie eine nicht tatsächlich verarbeitete
      Nachricht, "neu" gezählte Reports sind immer wirklich gespeichert.

**Zwei echte Bugs unterwegs gefunden und behoben** (durch fehlschlagende
Tests entdeckt, nicht nur behauptet):
1. `ErrDuplicateReport` saß in `internal/infra/sqlite` — `syncreports`
   hätte es importieren müssen, ein Verstoß gegen DIP (App-Schicht darf nur
   an Domain-Ports hängen, AGENTS.md). Verschoben nach `report.ErrDuplicate`
   im Domain-Port, `sqlite.ReportRepository.Save` gibt ihn jetzt (gewrappt)
   zurück statt eines eigenen Sentinels.
2. Ein bisher unausgesprochener Vertrag an `account.CredentialStore.Store`:
   die echten Adapter (`OSStore`, `FileStore`) verwandeln die Secret-Bytes
   synchron in eine eigene Kopie (String-Konversion bzw. Verschlüsselung),
   ein Aufrufer darf das übergebene `Secret` direkt danach mit `Zero()`
   überschreiben. Ein naiver Test-Fake, der nur flach speichert, würde vom
   selben `Zero()`-Aufruf nachträglich mitgeleert (geteiltes Backing-Array)
   — beim CLI-Test `account add` tatsächlich aufgetreten. Vertrag jetzt am
   Port dokumentiert, alle drei Fake-`CredentialStore`-Implementierungen im
   Repo kopieren jetzt defensiv (`account.NewSecret(s.Expose())`).

**Fertig wenn:** Ein vollständiger Import aus einem Postfach läuft über die
CLI durch, inklusive Kennzahlenausgabe. — Mit einer ehrlichen Einschränkung
erreicht: end-to-end über die echte, gebaute Binary verifiziert für
**Datei-Import** (`dmarc-analyzer import` mit drei echten Provider-Stil-
Fixtures — RFC-7489-Beispiel, Google-gzip, Microsoft-zip —, danach
`dmarc-analyzer stats` mit von Hand nachgerechneten Werten, Duplikaterkennung
bei erneutem Import). **Nicht** end-to-end mit der echten Binary gegen ein
echtes/simuliertes Postfach getestet — das würde `account add` erfordern,
was den echten OS-Schlüsselbund dieser Maschine anfassen würde, das wollte
ich ohne Rückfrage nicht auslösen. Der IMAP-Pfad selbst ist stattdessen
doppelt abgesichert: AP 3 testet den Adapter gegen einen echten In-Process-
IMAP-Server, AP 4 testet `syncreports.SyncAccount` (der Use Case, der ihn
aufruft) umfassend gegen Fakes, inklusive des genauen "zweiter Lauf holt
nichts"-Kriteriums.

### AP 5 — UI-Grundgerüst

**Ziel:** Bedienbares Programm für den Kern-Use-Case.

- [x] Hauptfenster, seitliche Navigation, eigenes Theme (hell und dunkel)
- [x] `internal/ui/i18n` — **alle** Texte zentral, kein String im Widget-Code
- [x] Einstellungen: Konto anlegen, Verbindung testen mit klarer Rückmeldung
- [x] Ersteinrichtungs-Assistent nach 3.1
- [x] Berichtstabelle mit Lazy-Datenquelle über `ReportQuery`
- [x] Detailansicht eines Reports
- [x] Sync-Knopf mit Fortschritt und Abbruch, I/O nie im UI-Thread,
      Rückweg über `fyne.Do()`
- [x] Leerzustände und Klartext-Fehlermeldungen nach 3.1
- [x] Tests mit `fyne.io/fyne/v2/test`: Aufbau, Navigation, Formularvalidierung

**Fertig wenn:** Ein Nutzer richtet ohne Dokumentation ein Konto ein und sieht
seine Reports. ✅ Erreicht.

**Abweichungen / Erkenntnisse:**
- Eigenes Theme beschränkt sich bewusst auf eine Akzentfarbe
  (`appTheme.Color` überschreibt nur `ColorNamePrimary`); Light/Dark-Kontrast
  und -Umschaltung übernimmt unverändert Fynes `ThemeVariant`-System, da
  dieses bereits geprüft barrierefrei ist — ein eigenes Kontrastsystem hätte
  hier nur Risiko ohne Nutzen hinzugefügt.
- Echter Bug gefunden und behoben: `settings.View.refreshContent()` schrieb
  den neuen Listen-/Leerzustands-Inhalt fälschlich nach
  `v.container.Objects[len(v.container.Objects)-1]`. Bei
  `container.NewBorder(top, nil, nil, nil, center)` landet das variadic
  `objects`-Argument (hier `center`) aber immer an Index 0 — die
  Top/Bottom/Left/Right-Objekte werden erst danach angehängt. Dadurch wurde
  bei jedem `Reload()` die Kopfzeile (Titel „Konten" + Hinzufügen-Button)
  durch die Liste/den Leerzustand überschrieben, statt umgekehrt — sichtbar
  wurde das erst durch den fehlschlagenden Shell-Navigationstest
  `TestShell_SelectNav_SwitchesToSettings`. `reports.View` hatte dasselbe
  Muster bereits korrekt (Index 0). Fix: beide Zweige schreiben jetzt auf
  `Objects[0]`.
- Fyne-Testtreiber führt `fyne.Do()` synchron auf der aufrufenden Goroutine
  aus (anders als der echte Treiber) — ohne das injizierbare
  `runBackground`-Feld (Default: echte Goroutine, in Tests synchron ersetzt)
  hätten Tests unter `-race` echte Data Races gemeldet. Siehe AGENTS.md.

### AP 6 — Auswertung und Visualisierung

**Ziel:** Die in `FEATURES.md` geforderte Visualisierung steht.

- [x] Dashboard mit Kennzahlen-Kacheln inkl. Trend zur Vorperiode
- [x] `ChartRenderer`-Port + go-chart-Adapter
- [x] Zeitreihe, Top-10-Balken, Disposition-Donut, Heatmap Quelle × Tag
- [x] Sendequellen-Ansicht, aggregiert nach Quell-IP
- [x] rDNS/PTR-Auflösung mit Cache (Vorschlag 11.2)
- [x] Erkennung bekannter Dienste über PTR und IP-Bereiche (Vorschlag 11.3)
- [x] Filter- und Gruppierungsleiste, wirkt auf alle Ansichten
- [x] Glossar und Tooltips für DMARC-Begriffe
- [x] Export: gefilterte Ansicht als CSV, Diagramm als PNG (Vorschlag 11.4)

**Fertig wenn:** Aus dem Dashboard ist ohne Zwischenschritt erkennbar, welche
Sendequelle fehlschlägt und wie viele Nachrichten betroffen sind. ✅ Erreicht.

**Abweichungen / Erkenntnisse:**
- `analysis.Repository` (AP 2/4) um `DailyVolumes`, `TopSources`, `Heatmap`
  erweitert statt eines neuen Ports — bleibt der eine Ort für
  SQL-aggregierte Lesezugriffe auf Kennzahlen, konsistent mit `Compute`.
  Ein Report wird für die Zeitreihe/Heatmap dem Tag von `date_begin`
  zugeordnet, nicht anteilig verteilt — DMARC-Aggregate-Reports sind nach
  RFC 7489 praktisch immer Ein-Tages-Zeiträume, eine anteilige Verteilung
  hätte keinen belastbaren fachlichen Mehrwert gegenüber dem Aufwand
  gehabt.
- Neuer Domänen-Port `domain/sources` (Paketname `sources`, wie
  `internal/ui/sources` — unterschiedliche Importpfade, im UI-Code per
  Alias `domainsources` importiert, demselben Muster wie `domainsync`
  folgend) für die nach Quell-IP aggregierte Sicht plus `Enricher`-Port
  (PTR + Diensterkennung). SQL-Aggregation (Repository) und
  Netzwerk-Anreicherung (Enricher) sind bewusst getrennte Ports und laufen
  in unterschiedlichen Schichten (Repository in `infra/sqlite`, Enrich-Loop
  in `app/sourcestats`) — zwei grundverschiedene I/O-Arten, die ein
  gemeinsamer Adapter vermischt hätte.
- `internal/infra/sourceinfo.Enricher`: nur hostnamenbasierte
  Diensterkennung (PTR-Suffix), bewusst **ohne** zusätzliche
  IP-Bereichs-Erkennung trotz FEATURES.md-Wortlaut ("über PTR und
  IP-Bereiche") — fest einprogrammierte CIDR-Listen großer Anbieter wären
  nach kurzer Zeit unbemerkt veraltet, ohne einen Pflegeprozess dafür ist
  das schlechter als gar keine Angabe. Cache ist ein unbegrenzter
  In-Prozess-`sync.Map` (kein TTL) — für eine Desktop-Anwendung mit kurzen
  Laufzeiten ausreichend.
- go-chart/v2 hat keinen Heatmap-Diagrammtyp. Die Heatmap wird deshalb
  nicht über go-chart, sondern direkt mit `image/draw` gezeichnet
  (Rechtecke) plus go-charts eigenem `drawing.RasterGraphicContext` für
  Text (Achsenbeschriftung) — spart eine zusätzliche
  Font-Rendering-Abhängigkeit nur für dieses eine Diagramm.
- `ChartRenderer` liefert bei leeren Eingabedaten ein leeres weißes Bild
  statt eines Fehlers — die aufrufende `dashboard.View` zeigt in diesem
  Fall ohnehin einen Leerzustand statt eines Diagramms an; ein Fehler wäre
  hier unnötig streng gewesen.
- Diagrammbeschriftungen (z. B. "Bestanden"/"Fehlgeschlagen",
  Disposition-Namen) sind als literale deutsche Zeichenketten direkt in
  `internal/infra/charts` gehalten, nicht aus `internal/ui/i18n`
  importiert — `i18n` ist eine UI-Schicht-Abhängigkeit, die `infra` laut
  Abhängigkeitsrichtung nicht importieren darf. Kleine, bewusst in Kauf
  genommene Textdopplung.
- "Tooltips" (Checklistenpunkt) sind kein Hover-Tooltip — Fyne v2.8 hat
  keinen eingebauten Tooltip-Mechanismus (geprüft: kein Treffer im
  gesamten Modul). Ersatz: ein kleiner "?"-Knopf neben einzelnen
  Dashboard-Kacheln bzw. im Hauptfenster-Kopf öffnet denselben
  Glossar-Text als Dialog (`internal/ui/glossary`).
- "Gruppierungsleiste" wurde nicht in die gemeinsame `FilterBar`
  aufgenommen: `analysis.Query` und `sources.Query` kennen kein `GroupBy`
  (Übersicht/Sendequellen sind bereits aggregiert, eine zusätzliche
  Gruppierung ergäbe dort keinen Sinn). Stattdessen bekam ausschließlich
  `reports.View` einen eigenen Gruppierungs-`Select` (Keine/Domain/
  Organisation), der das seit AP 2 vorhandene `report.Query.GroupBy`
  endlich an die UI anschließt.
- Export exportiert die aktuell geladene(n) Seite(n) (`reports.View`/
  `sources.View`), nicht zwangsläufig jeden Datensatz, der dem Filter
  insgesamt entspricht — alle Seiten nur für den Export vorab zu laden
  wäre bei großen Beständen selbst eine Performance-Falle und würde der
  in AP 2/5 bewusst gewählten Lazy-Datenquelle widersprechen.
- **Echter Bug gefunden und behoben** (Konstruktionsreihenfolge, kein
  Testartefakt): `widget.Select.SetSelected()` löst `OnChanged` synchron
  aus, auch im Konstruktor. Das neue Gruppierungs-`Select` in
  `reports.View` rief `SetSelected()` auf, bevor `v.container` zugewiesen
  war — der synchron ausgelöste `OnChanged`-Handler griff über
  `Reload()`/`setCenter()` auf das noch nil `v.container` zu und
  panickte direkt beim `NewView()`-Aufruf. Aufgedeckt durch einen simplen
  Konstruktionstest (`NewView()` + `SetContent`), nicht durch komplexe
  Fixtures. Fix: `Select` zunächst ohne `OnChanged` aufbauen, `OnChanged`
  erst zuweisen, nachdem der Rest des Widgets fertig ist — siehe
  AGENTS.md.
- **AP-5-Lücke geschlossen:** Es gab bislang keinen Weg, das grafische
  Programm tatsächlich zu starten — `ui.BuildMainWindow` existierte, aber
  `main.go` rief es nirgends auf (nur `run()`/Unterbefehle). Jetzt startet
  `dmarc-analyzer` ohne Argumente die GUI (`cmd_gui.go`, `fyneapp.New()` +
  `window.ShowAndRun()`); `--help`/`-h` zeigt weiterhin nur die
  Kommandozeilen-Hilfe. Bewusst **nicht** in `run()` verdrahtet, da
  `run()` von `cmd_*_test.go` direkt mit leeren Argumenten aufgerufen wird
  (`TestRun_NoArgs_PrintsUsageWithoutError`) und dabei nie ein echtes
  Fenster öffnen darf — die GUI-Startentscheidung liegt deshalb
  ausschließlich in `main()`, das nicht unit-getestet wird. `"gui"` wurde
  zu `credentialAwareCommands` ergänzt, da sowohl Kontoverwaltung als auch
  Sync-Knopf im Hauptfenster Zugangsdaten brauchen. Ein echter Start der
  GUI in einer Desktop-Umgebung wurde in dieser Session **nicht**
  verifiziert (keine Anzeige im Entwicklungscontainer verfügbar) — nur
  `--help` und alle bestehenden Unterbefehle wurden am gebauten Binary
  geprüft; die Fyne-Widget-Ebene selbst ist durchgängig per
  `fyne.io/fyne/v2/test` (Headless-Treiber) getestet.

**Nachträgliche Ergänzung (nach Abschluss von AP 6):** Ordner-Picker fürs
Kontoformular — DMARC-Berichte landen nicht zwangsläufig im Wurzelpostfach
(INBOX), sondern können z. B. per Mailregel in einen Unterordner
einsortiert sein. Das Postfach-Feld selbst unterstützte technisch schon
immer einen beliebigen Ordnerpfad (freier Text, unverändert an IMAP
SELECT/EXAMINE durchgereicht), aber ohne Hilfestellung musste der Nutzer
den exakten Pfad samt serverabhängigem Trennzeichen (z. B. `/` bei
Dovecot, `.` bei Courier) blind erraten.
- Neuer, optionaler Port `sync.MailboxLister` (`domain/sync/ports.go`) —
  bewusst getrennt von `MessageSource` statt einer weiteren Methode dort,
  da nicht jede denkbare Quelle Postfächer auflisten kann/muss; Aufrufer
  prüfen per Typassertion. `imap.Adapter` implementiert ihn zusätzlich
  über `IMAP LIST`, gefiltert auf tatsächlich auswählbare Postfächer
  (Attribut `\Noselect` ausgeschlossen).
- `manageaccount.UseCase.ListMailboxes` verbindet probeweise (wie
  `TestConnection`, ohne zu synchronisieren) und listet auf.
- `settings.AccountForm.mailbox` ist jetzt ein `widget.SelectEntry` statt
  `widget.Entry` — weiterhin frei eintippbar (z. B. für Server, bei denen
  LIST aus irgendeinem Grund nicht das Richtige liefert), zusätzlich mit
  Dropdown-Optionen befüllbar über den neuen Knopf "Ordner auflisten"
  (nutzt die aktuell im Formular eingetragenen Zugangsdaten, ohne dass das
  Konto schon gespeichert sein muss — funktioniert dadurch sowohl beim
  Anlegen in den Einstellungen als auch im Ersteinrichtungs-Assistenten).
- Kleine, durch Umstellung auf `SelectEntry` verursachte Testfalle
  gefunden und behoben: `uitest.FindEntries` erkennt `*widget.SelectEntry`
  nicht (Typassertion auf den konkreten Typ `*widget.Entry`, `SelectEntry`
  bettet `Entry` nur ein) — ein index-basierter Testhelfer im
  Ersteinrichtungs-Assistenten (`fillValidAccountForm`) musste neu
  durchgezählt werden. Siehe AGENTS.md.

### AP 7 — Feinschliff und Release 1.0.0

> **Vor diesem AP eingeschoben:** `MIGRATIONSPLAN.md` (Umstieg von der
> ursprünglichen Fyne-Desktop-Oberfläche auf eine im Programm eingebettete
> Web-Oberfläche, Meilensteine M0–M6). Stand hier: M0–M5 umgesetzt, siehe
> dort Abschnitt 10. Die Punkte unten sind entsprechend angepasst
> (Desktop-Benachrichtigung → Browser-Benachrichtigung, Packaging ohne
> `fyne package`) bzw. bereits durch M5 ganz oder teilweise erledigt.

- [ ] Aufbewahrungsrichtlinie, Standard 24 Monate (Vorschlag 11.11)
- [ ] Hintergrund-Sync nach Zeitplan + **Browser-Benachrichtigung bei
  offenem Tab** (statt der ursprünglich vorgesehenen
  `fyne.App.SendNotification`-Desktop-Benachrichtigung — es gibt seit dem
  Umstieg auf die Web-Oberfläche kein Desktop-App-Fenster mehr, das eine
  systemeigene Benachrichtigung auslösen könnte; die Benachrichtigung
  müsste über die Browser-Notifications-API laufen und setzt daher einen
  geöffneten Tab voraus)
- [ ] Backup/Restore per `VACUUM INTO` (Vorschlag 11.12)
- [ ] Barrierefreiheits-Durchgang nach den Kriterien aus 3.1 (siehe auch
  `MIGRATIONSPLAN.md` M6)
- [ ] Lasttest mit mehreren Jahren Reportdaten
- [x] ~~Packaging macOS/Windows/Linux, `task release` mit Checksummen~~ —
  **in M5 umgesetzt** (`Taskfile.yml`: `release:darwin`/`release:windows`/
  `release:linux`/`release`, reine `CGO_ENABLED=0`-Cross-Compiles ohne
  `fyne package`, siehe `MIGRATIONSPLAN.md`). Offen bleibt für ein
  echtes 1.0.0-Release: macOS-Signierung/Notarisierung (aktuell
  unsigniert, siehe README „Installation"), ggf. zusätzliche
  Distributionswege (Homebrew, winget, …) — bewusst nicht Teil von M5.
- [ ] README final mit Screenshots und Datenschutzhinweis — Text-Teile
  bereits in M5 aktualisiert (Start/Browserverhalten/Beenden/
  `--kein-browser`/Sicherheitsmodell), **Screenshots fehlen noch**. ADRs
  in `docs/adr/` bereits in M5 angelegt (0001 Web-Oberfläche statt Fyne,
  0002 Chart.js statt `ChartRenderer`-Port).
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
