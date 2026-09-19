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
- Moderneres, vollständig eigenes Farbschema (`internal/ui.appTheme`) für
  hellen und dunklen Modus, nach der validierten Referenzpalette der
  `dataviz`-Skill (Primärblau, feste Statusfarben Grün/Gelb/Rot) — löst
  das gemeldete "altbacken, alles schwarz" im dunklen Systemmodus, das
  vorher fast vollständig an Fynes generisches Standard-Theme delegiert
  wurde. Dieselben Statusfarben jetzt auch in
  `internal/infra/charts` (vorher ad-hoc gewählte Grün-/Rot-Töne) —
  Oberfläche und Diagramme sprechen dieselbe Farbsprache. Größere
  Eckenradien/Abstände (Karten, Knöpfe, Eingabefelder) für einen
  luftigeren, weniger kantigen Eindruck.
- Dashboard-Kacheln und -Diagramme stecken jetzt in `widget.Card` statt
  frei auf dem Fensterhintergrund zu stehen, mit Symbolen je Kennzahl.
  Die vier Diagramme sind neu gruppiert statt einer langen Spalte:
  Zeitreihe und Disposition-Donut nebeneinander, Top-Sendequellen und
  Heatmap je mit eigener voller Zeile und horizontalem Scrollbereich
  (ihre Breite wächst mit der Anzahl Sendequellen/Tagen). Seitliche
  Navigation jetzt mit Symbolen je Eintrag, Trennlinien zwischen
  Kopfzeile/Navigation und dem Inhalt.
- Echter Bug behoben: Der Bericht-Detaildialog (Berichte-Tabelle, Zeile
  antippen) ließ sich nicht mehr schließen. Er lief auf
  `dialog.NewCustomWithoutButtons` — ganz ohne Knopf, und kein Code-Pfad
  rief `Hide()` auf. Anders als ein gewöhnliches Popup schließt das
  zugrunde liegende `widget.ModalPopUp` nicht durch Antippen außerhalb,
  der Dialog blieb also dauerhaft offen. Jetzt `dialog.NewCustom(...)`
  mit einem "Schließen"-Knopf.
- `DMARC_OIDC_INSECURE_SKIP_VERIFY` (optional, Vorgabe `false`): schaltet
  die TLS-Zertifikatsprüfung für alle Calls gegen den OIDC-Issuer
  (Discovery, JWKS, Token-Exchange) ab — ausschließlich für
  Entwicklungsumgebungen mit selbstsigniertem Zertifikat gedacht (z. B.
  Caddys `tls internal`), niemals für Produktion. Siehe
  `docs/features/deployment.md`.

### Geändert

- Migration der gesamten Präsentationsschicht von der Fyne-Desktop-
  Oberfläche zu einer im Programm eingebetteten Web-Oberfläche
  (`internal/web`, `MIGRATIONSPLAN.md` Meilensteine M0–M5): Beim Start
  läuft ein lokaler HTTP-Server auf `127.0.0.1`, der Standardbrowser
  öffnet automatisch eine Seite mit Einmal-Anmeldelink (60 s gültig, gegen
  ein `HttpOnly`/`SameSite=Strict`-Sitzungs-Cookie eingetauscht).
  Einzelinstanz-Erkennung über `instance.json`: ein zweiter Start holt
  sich nur einen frischen Anmeldelink. Sicherheitsmodell: `Host`-Prüfung
  gegen DNS-Rebinding, CSRF-Token- und `Origin`/`Sec-Fetch-Site`-Prüfung
  bei jeder zustandsändernden Anfrage, Content-Security-Policy ohne
  `unsafe-inline`. Lebenszyklus ausschließlich über Strg+C/SIGTERM (kein
  Auto-Ende, kein „Beenden"-Knopf) — siehe README „Start und Beenden".
- Alle vier Dashboard-Diagramme (Zeitreihe, Top-Sendequellen, Disposition,
  Heatmap) werden jetzt clientseitig mit Chart.js gezeichnet
  (`internal/web/static/charts.js`, Plugins `chartjs-chart-matrix` für die
  Heatmap und `chartjs-plugin-zoom` für die Zeitreihe) statt serverseitig
  als PNG (`go-chart`) — dadurch Tooltips, umschaltbare Legende, Zoom in
  der Zeitreihe und Klick-Drilldown (ein Klick auf Tag/Quelle/Zelle/
  Segment öffnet die passend gefilterten Berichte). Jedes Diagramm hat
  zusätzlich eine zuschaltbare Tabellenansicht (Barrierefreiheit) sowie
  PNG-/CSV-Export-Knöpfe.
- Berichte- und Sendequellen-Tabellen laufen jetzt über seitenweises
  Nachladen mit htmx statt einer virtualisierten Fyne-Tabelle; CSV-Export
  lädt den gesamten gefilterten Bestand gestreamt nach, ohne ihn
  vollständig im Speicher zu halten.
- Import (`importfiles.UseCase.ImportData`) jetzt zusätzlich per Web-
  Upload (`POST /import`, Drag & Drop, mehrere Dateien gleichzeitig,
  50 MB je Datei) statt nur per CLI-Dateipfad.
- Sync läuft jetzt als serverseitiger Hintergrund-Auftrag
  (`internal/app/syncjob`, höchstens ein Lauf gleichzeitig, Abbruch per
  Kontext) mit Live-Fortschritt über Server-Sent Events
  (`GET /ereignisse`) statt eines synchronen Fyne-Fortschrittsdialogs.
- Zugangsdaten-Sperre: `account.CredentialStore` kann jetzt gesperrt sein
  (`account.ErrCredentialStoreLocked`) — die Oberfläche leitet dann auf
  eine Entsperr-Seite (`/entsperren`) für die Master-Passphrase des
  Datei-Fallback-Schlüsselspeichers (Linux ohne Secret Service), statt
  wie zuvor auf der Konsole danach zu fragen (die es beim Start ohne
  Terminal, z. B. per Doppelklick, nicht gibt).

### Entfernt

- `internal/ui` (gesamte Fyne-Oberfläche, ~4.900 Zeilen inkl. Tests),
  `cmd/dmarc-analyzer/cmd_gui.go` sowie `fyne.io/fyne/v2` und alle
  transitiven Fyne-Abhängigkeiten aus `go.mod` (Meilenstein M5). Der
  versteckte Übergangs-Unterbefehl `gui` (Vergleichs-/Rückfallpfad
  während der Migration) entfällt damit ebenfalls.
- `internal/infra/charts` (`go-chart`-Implementierung des
  `ChartRenderer`-Ports), `analysis.ChartRenderer` selbst sowie
  `exportdata.WriteChartPNG` — Diagramme entstehen jetzt vollständig im
  Browser (siehe „Geändert" oben). `github.com/wcharczuk/go-chart/v2`
  damit ebenfalls aus `go.mod` entfernt. Begründung in
  `docs/adr/0001-web-oberflaeche-statt-fyne.md` und
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.
- Damit ist das gesamte Modul jetzt frei von CGO-Abhängigkeiten
  (`go list -deps ./... | grep fyne` liefert nichts mehr); `task release`
  baut alle Zielplattformen (macOS arm64/amd64, Windows amd64, Linux
  amd64/arm64) als reine `CGO_ENABLED=0`-Cross-Compiles auf einem
  einzigen Rechner, ohne `fyne package` oder plattformspezifische
  Toolchains — siehe `Taskfile.yml` (`release:darwin`/`release:windows`/
  `release:linux`/`release`) und die neue GitHub-Actions-Pipeline
  (`.github/workflows/ci.yml`: `task check` bei jedem Push/PR, `task
  release` samt GitHub-Release bei Versions-Tags).

### Behoben

- `.zip`-Anhänge mit mehreren enthaltenen DMARC-Reports wurden sowohl
  beim Datei-Import als auch beim IMAP-Sync nur zu einem Report
  importiert (`ReportParser.Parse()` statt `ParseAll()`) — behoben über
  eine neue, optionale Schnittstelle `domainsync.MultiReportParser` und
  eine gemeinsame Hilfsfunktion `domainsync.ParseAttachment`.
- `syncjob.Runner` meldete einen Abbruch während des letzten Kontos
  fälschlich als `done`.
- `/entsperren` akzeptierte jede Passphrase, wenn noch kein Konto
  existierte.
- Ein Chart.js-Diagramm (Nachrichtenvolumen pro Tag) wuchs beim Laden des
  Dashboards unbegrenzt nach unten — klassischer Chart.js-
  Rückkopplungsschleifen-Bug bei `responsive: true` +
  `maintainAspectRatio: false` ohne Wrapper-Element mit fester,
  vom Canvas unabhängiger Höhe (siehe `AGENTS.md`).
