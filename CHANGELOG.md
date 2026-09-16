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
- UI-Grundgerüst (AP 5, `internal/ui`): Hauptfenster mit seitlicher
  Navigation (Berichte/Einstellungen) und eigenem Theme (Akzentfarbe, hell
  und dunkel über Fynes `ThemeVariant`-System). `internal/ui/i18n` bündelt
  alle sichtbaren Texte zentral. `settings.View` zum Anlegen, Testen und
  Löschen von Konten. Dreistufiger Ersteinrichtungs-Assistent
  (`onboarding.Wizard`: Konto → Verbindungstest → erster Abgleich).
  `reports.View` mit lazy-ladender Berichtstabelle über `ReportQuery`
  (Seitengröße 50) und Detailansicht. Sync-Knopf mit Fortschrittsanzeige
  und Abbruch, I/O ausschließlich über injizierbares `runBackground`
  außerhalb des UI-Threads, Rückweg über `fyne.Do()`. Leerzustände und
  Klartext-Fehlermeldungen für alle Ansichten. Vollständig mit
  `fyne.io/fyne/v2/test` getestet (Aufbau, Navigation, Formularvalidierung),
  `-race`-sauber dank injizierbarem `runBackground`.
- Echter Bug behoben: `settings.View.refreshContent()` überschrieb bei
  jedem `Reload()` die Kopfzeile (Titel + Hinzufügen-Button) statt der
  Liste, weil bei `container.NewBorder` der Center-Slot an Index 0 liegt,
  nicht am Ende von `.Objects` — aufgedeckt durch den Navigationstest
  `TestShell_SelectNav_SwitchesToSettings`.
- Auswertung und Visualisierung (AP 6): Übersicht (`internal/ui/dashboard`)
  mit Kennzahlen-Kacheln inklusive Trendpfeil zur Vorperiode sowie vier
  Diagrammen (Zeitreihe gestapelt nach Pass/Fail, Top-10-Sendequellen nach
  Volumen und Pass-Rate eingefärbt, Disposition-Donut, Quelle-×-Tag-
  Heatmap). `analysis.ChartRenderer`-Port gegen
  `github.com/wcharczuk/go-chart/v2` implementiert
  (`internal/infra/charts`); die Heatmap wird mangels go-chart-Unter-
  stützung direkt mit `image/draw` gezeichnet. `analysis.Repository` um
  `DailyVolumes`/`TopSources`/`Heatmap` erweitert (SQL-Aggregation wie
  `Compute`), `statistics.UseCase.Dashboard` bündelt alle Übersichtsdaten
  in einem Ladevorgang.
- Sendequellen-Ansicht (`internal/ui/sources`): nach Quell-IP aggregierte,
  lazy-ladende Tabelle (Volumen, Pass-Rate, PTR-Hostname, erkannter
  Dienst) über den neuen Domänen-Port `domain/sources`
  (`SourceStatsRepository` in `internal/infra/sqlite`, Keyset-Pagination
  wie bei Reports). rDNS/PTR-Auflösung mit Prozess-Cache sowie
  hostnamenbasierte Erkennung bekannter Diensteanbieter (Google
  Workspace, Microsoft 365, Mailchimp, SendGrid, Brevo, Postmark) in
  `internal/infra/sourceinfo` — bewusst ohne zusätzliche
  IP-Bereichs-Listen, siehe UMSETZUNGSPLAN.md für die Begründung.
- Gemeinsame Filterleiste (`components.FilterBar`: Zeitraum-Voreinstellung
  + Domain) wirkt jetzt auf Übersicht, Berichte und Sendequellen
  gleichzeitig. `reports.View` bekam zusätzlich einen eigenen
  Gruppierungs-`Select` (Keine/Domain/Organisation), der das seit AP 2
  vorhandene `report.Query.GroupBy` erstmals an die UI anschließt.
- Glossar (`internal/ui/glossary`) mit den wichtigsten DMARC-Begriffen,
  erreichbar über einen Kopfzeilen-Knopf sowie kleine "?"-Knöpfe an
  einzelnen Kennzahlen-Kacheln — Ersatz für Hover-Tooltips, die Fyne v2.8
  nicht unterstützt.
- Export: `exportdata.WriteChartPNG` (Diagramme als PNG) und
  `exportdata.WriteSourceStatsCSV` (Sendequellen als CSV) ergänzen den
  bestehenden Report-/Record-CSV-Export; `reports.View`/`sources.View`/
  jedes Diagramm-Panel haben je einen Export-Knopf.
- Das grafische Programm ist jetzt tatsächlich startbar: `dmarc-analyzer`
  ohne Argumente öffnet das Hauptfenster (`cmd_gui.go`), `--help`/`-h`
  zeigt die Kommandozeilen-Hilfe. Vorher existierte `ui.BuildMainWindow`
  zwar vollständig getestet, war aber nirgends verdrahtet — eine seit
  AP 5 offene Lücke.
- Echter Bug behoben (Konstruktionsreihenfolge): `widget.Select.
  SetSelected()` löst `OnChanged` synchron aus, auch im Konstruktor. Das
  neue Gruppierungs-`Select` in `reports.View` griff dadurch beim
  `NewView()`-Aufruf über den vorzeitig ausgelösten Handler auf das noch
  nicht zugewiesene `v.container` zu (Nil-Pointer-Panic) — aufgedeckt
  durch einen einfachen Konstruktionstest.
- Ordner-Picker fürs Kontoformular: DMARC-Berichte landen nicht
  zwangsläufig im Wurzelpostfach — ein neuer, optionaler Port
  `sync.MailboxLister` (implementiert von `imap.Adapter` über IMAP LIST,
  ohne `\Noselect`-Postfächer) plus `manageaccount.UseCase.ListMailboxes`
  lassen das Kontoformular (Einstellungen und Ersteinrichtungs-Assistent)
  die auf dem Server tatsächlich vorhandenen Postfächer/Unterordner
  auflisten. Das Postfach-Feld ist jetzt ein `widget.SelectEntry`
  (weiterhin frei eintippbar, zusätzlich mit den gefundenen Ordnern als
  Dropdown).
