# Architektur

`dmarc-analyzer` folgt Clean Architecture: vier Schichten, Abhängigkeiten
zeigen ausschließlich nach innen.

```
cmd/dmarc-analyzer  →  internal/web  →  internal/app  →  internal/domain
                        internal/infra ─────────────────↗
```

## Schichten

- **`internal/domain`** — reine Fachlogik: Aggregate (`report.AggregateReport`,
  `account.MailAccount`), Wertobjekte, Validierung, Domänen-Ports (Interfaces wie `report.Repository`,
  `sync.MessageSource`). Keine
  Imports außerhalb der Standardbibliothek — kein `encoding/xml`, kein SQL,
  kein `net/http` hier.
- **`internal/app`** — Use Cases, die eine fachliche Aufgabe orchestrieren (`syncreports.UseCase.SyncAccount`,
  `retention.UseCase.ApplyNow`,
  `manageaccount.UseCase.TestConnectionByID`). Hängen nur an
  Domänen-Ports, nie an einen konkreten Adapter.
- **`internal/infra`** — Adapter, die Domänen-Ports gegen konkrete Technik
  implementieren: `sqlite` (Persistenz), `imap` (IMAP-Zugriff), `dmarcxml`
  (RFC-7489-Parser), `mailmime` (MIME-Zerlegung), `sourceinfo`
  (PTR-/Dienst-Erkennung), `envconfig` (liest die gesamte
  Laufzeit-Konfiguration aus Umgebungsvariablen, siehe
  `docs/features/deployment.md`).
- **`internal/web`** — die Ausliefer-Schicht: `net/http` + `html/template`
  (kein Bundler, kein Node-Werkzeug) für serverseitig gerenderte Seiten,
  Chart.js für Diagramme (`static/charts.js` bekommt fertig aufbereitete
  JSON-Daten aus Go, bleibt selbst dünn), sowie der OIDC-Login-Flow (`auth.go`/`oidc.go`, siehe
  `docs/features/auth.md`). Ruft
  ausschließlich Use Cases aus `internal/app` auf, nie direkt einen
  Infra-Adapter.
- **`cmd/dmarc-analyzer`** — die Composition Root: einziger Ort, an dem
  konkrete Adapter mit Use Cases verdrahtet werden (`wire.go`), liest hier
  auch die ENV-Konfiguration (`envconfig.Load()`) und startet je nach
  Unterbefehl (`web`, `sync`, `import`, `stats`, `healthcheck`) den
  passenden Pfad.

## Warum diese Trennung

- **Domain unabhängig testbar** — Geschäftsregeln (z. B. "wann gilt eine
  Nachricht als DMARC-konform", RFC-7489-Feld-Defaults) lassen sich ohne
  Datenbank, ohne Netzwerk, ohne HTTP-Server prüfen.
- **Austauschbare Adapter** — `internal/app` kennt nur Interfaces wie
  `report.Repository`; Tests setzen dort In-Memory-Fakes ein, Produktion
  echtes SQLite. Ein Wechsel der Datenbank würde nur `internal/infra/sqlite`
  betreffen, keine Zeile in `internal/domain`/`internal/app`.
- **Ein einziger Ort kennt die konkrete Technik** — nur `cmd/dmarc-analyzer`
  importiert gleichzeitig `internal/infra/sqlite`, `internal/infra/imap`
  usw. *und* baut daraus die Use Cases zusammen. Das hält
  `internal/app`/`internal/web` frei von technischen Details, die dort
  nicht hingehören.

## Optionale Ports statt großer Interfaces

Mehrere Ports sind bewusst klein und optional zugeschnitten statt in ein
großes Interface gepackt zu werden — ein Aufrufer prüft per
Typ-Assertion, ob ein Adapter die Zusatzfähigkeit hat:

- `domainsync.MailboxLister` — optionale Erweiterung von
  `domainsync.MessageSource`, die auf dem Server vorhandene Postfächer
  auflisten kann.
- `domainsync.MultiReportParser` — optionale Erweiterung von
  `domainsync.ReportParser` für Anhänge mit mehreren Reports (z. B. ein
  `.zip` mit mehreren XML-Dateien).
- `report.Pruner` — eigener, schmaler Port nur für die
  Aufbewahrungsrichtlinie (`DeleteOlderThan`), getrennt von
  `report.Repository`, weil Löschen nach Alter eine reine
  Wartungsoperation ist, die nur `internal/app/retention` braucht.

## Sync-Pipeline (`internal/app/syncreports`)

Abholen und Parsen laufen nebenläufig (ein Worker-Pool dekodiert/parst
MIME-Anhänge parallel), Schreiben nach SQLite läuft **seriell** über eine
einzige Goroutine — SQLite verträgt keine konkurrierenden Schreiber gut.
Der Fortschritt (`sync.State`, zuletzt verarbeitete UID) wird nur über eine
lückenlose Grenze fortgeschrieben, damit ein Abbruch mitten im Lauf nie
eine Nachricht überspringt, sondern höchstens eine bereits verarbeitete
Nachricht beim nächsten Lauf erneut anfasst (der `UNIQUE`-Index auf
`reports` fängt daraus resultierende Duplikate ab).

## Hintergrund-Aufträge (`internal/app/syncjob`, `syncscheduler`, `retentionjob`)

Drei unabhängige, langlebige Goroutinen laufen neben dem HTTP-Server (gestartet in `cmd/dmarc-analyzer/cmd_web.go`,
beendet über denselben
Kontext wie der Server):

- `syncjob.Runner` — höchstens ein manuell (`POST /abgleich`) oder geplant
  angestoßener Sync-Lauf gleichzeitig, Fortschritt per Server-Sent Events
  an beliebig viele Browser-Tabs.
- `syncscheduler.Scheduler` — löst `syncjob.Runner.Start()` alle
  `DMARC_SYNC_INTERVAL_MINUTES` aus (0 = deaktiviert).
- `retentionjob.Runner` — wendet die Aufbewahrungsrichtlinie einmal beim
  Start und danach alle 24 h an.
