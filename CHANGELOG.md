# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden hier dokumentiert.

Das Format folgt [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
die Versionierung folgt [Semantic Versioning](https://semver.org/lang/de/).

## [Unveröffentlicht]

### Hinzugefügt

- Projekt-Grundgerüst (AP 0): `go.mod`, Verzeichnisstruktur nach
  Clean-Architecture-Schichten, `Taskfile.yml`, `.golangci.yml`,
  GitHub-Actions-CI für macOS/Linux/Windows.
- Zentrales Logging (`internal/platform/logging`) und plattformkonforme
  Pfadauflösung (`internal/platform/paths`) für Datenbank, Konfiguration
  und Logs.
- Composition Root (`cmd/dmarc-analyzer`), startet und loggt die Version.
- Dokumentation der geplanten Abhängigkeiten mit gepinnten Versionen
  (`docs/DEPENDENCIES.md`).
- `README.md`, `LICENSE` (MIT), dieses Changelog.
- Domänenmodell für DMARC-Aggregate-Reports (AP 1): Aggregate Root
  `report.AggregateReport`, Value Objects (`SourceIP`, `DomainName`,
  `DateRange`, `Disposition`, `Policy`, `AlignmentMode`, `AuthResultValue`),
  `Record`, `PublishedPolicy` — alle mit erzwungenen Invarianten und ohne
  Abhängigkeiten außerhalb der Standardbibliothek. Unbekannte Enum-Werte aus
  der Praxis werden auf `Unknown` abgebildet statt verworfen.
- `report.Repository`-Port (Filtern/Sortieren/Gruppieren, Keyset-Pagination)
  und `sync.ReportParser`-Port, `sync.State` für den inkrementellen Sync.
- DMARC-XML-Parser (`internal/infra/dmarcxml`): entpackt `.xml`, `.xml.gz`
  und `.zip` (auch mit mehreren enthaltenen Reports), toleriert
  RFC-Abweichungen einzelner Provider (Groß-/Kleinschreibung, fehlendes
  `pct`/`adkim`/`aspf`), schützt mit einem 100-MB-Limit gegen Zip-/
  Gzip-Bomben.
- Golden-File-Tests gegen fünf synthetische Provider-Fixtures, Fuzz-Test
  (`FuzzParse`) sowie ein Test, der belegt, dass externe XML-Entities (XXE)
  nicht aufgelöst werden.
- SQLite-Persistenz (AP 2): eigener Migrator mit eingebetteten,
  nummerierten Migrationen; `internal/infra/sqlite.ReportRepository`
  implementiert `report.Repository` vollständig — `Save` (Transaktion,
  vorbereitete Batch-Statements), `Exists`/`ErrDuplicateReport` für
  Deduplizierung nach `(org_name, report_id, date_begin)`, `FindByID`
  (vollständig inkl. Records) und `Query` (Filter, Sortierung, Keyset-
  Pagination über SQLite-Row-Value-Vergleiche).
- Integrationstests gegen eine temporäre Datei-DB (WAL, echte
  Transaktionen) sowie ein Performance-Test, der 100.000 Records importiert
  und eine typische Berichtstabellen-Abfrage darauf misst (~7-8 ms).
- Mail & Sicherheit (AP 3): `internal/domain/account` (Aggregate
  `MailAccount`, Typ `Secret` mit strukturell erzwungener Maskierung über
  `String()`/`GoString()`/`MarshalJSON()`, Ports `Repository` und
  `CredentialStore`).
- IMAP-Adapter (`internal/infra/imap`) gegen `github.com/emersion/go-imap/v2`:
  context-fähiger TLS-Verbindungsaufbau, `EXAMINE`+`BODY.PEEK[]` (Postfach
  bleibt unberührt), UID-basierter streamender Iterator, automatischer
  Rescan bei `UIDVALIDITY`-Wechsel, Backoff mit 3 Versuchen für den
  Verbindungsaufbau (nicht für Login). Getestet gegen einen in-process
  IMAP-Server (`imapmemserver`), 11 Tests, 89 % Coverage.
- Keyring-Adapter (`internal/infra/keyring`): `OSStore` gegen den
  Betriebssystem-Schlüsselbund (Service `de.pmoscode.dmarc-analyzer`),
  `FileStore` als AES-256-GCM-Fallback mit scrypt-Schlüsselableitung für
  Systeme ohne Secret Service, `IsAvailable()` zur Laufzeit-Erkennung.
- Anwendungsschicht (AP 4): `syncreports.UseCase` orchestriert den
  kompletten inkrementellen Sync (verbinden, abholen, MIME zerlegen,
  parsen, deduplizieren, speichern, Fortschritt sichern) als nebenläufige
  Pipeline (Abholen/Parsen parallel, Schreiben seriell) mit korrekter
  Fortschritts-Fortschreibung auch bei außer der Reihe abgeschlossenen
  Nachrichten. `importfiles.UseCase` importiert dieselben Formate aus
  lokalen Dateien (.eml, .xml, .xml.gz, .zip) und teilt sich MIME-Zerlegung
  und Deduplizierung mit `syncreports`. `manageaccount.UseCase` verwaltet
  Konten vollständig (Anlegen, Verbindungstest, Löschen inkl.
  Schlüsselbund-Eintrag). `statistics.UseCase` berechnet Dashboard-
  Kennzahlen inklusive Vergleich zur Vorperiode. `queryreports.UseCase`
  und `exportdata` (CSV-Export) ergänzen die Anwendungsschicht.
- Neue SQLite-Adapter: `AccountRepository`, `SyncStateRepository`,
  `FailedImportRepository`, `StatisticsRepository` (SQL-Aggregation statt
  Laden einzelner Records).
- MIME-Zerlegung (`internal/infra/mailmime`) roher Nachrichten in Anhänge,
  gegen `github.com/emersion/go-message`.
- CLI (`cmd/dmarc-analyzer`): `sync`, `import <pfad>`, `stats`,
  `account add|list|test|delete`. Composition Root verdrahtet Adapter nur
  für den tatsächlich aufgerufenen Unterbefehl — `stats`/`import` fassen
  nie den OS-Schlüsselbund an.
- `report.ErrDuplicate` und `report.SaveIfNew` als gemeinsame,
  domänenseitige Deduplizierungslogik für alle Importwege.
