# Implementierungsplan — DMARC Analyzer

> Basis: `FEATURES.md`. Dieser Plan konkretisiert die Anforderungen zu einer
> umsetzbaren Architektur und einem Phasenplan.

---

## 1. Ausgangslage und Interpretation der Anforderungen

### 1.1 Begriffsklärung

In `FEATURES.md` steht durchgehend „DMerc". Das Repository heißt `dmarc-analyzer`,
und der beschriebene Ablauf (Daten aus einem Mailkonto abholen, auswerten,
visualisieren) entspricht exakt dem DMARC-Reporting-Workflow. **Annahme: gemeint ist
DMARC** (Domain-based Message Authentication, Reporting and Conformance, RFC 7489).

### 1.2 Zum Abschnitt „Non features"

Der Abschnitt „## Non features" in `FEATURES.md` listet inhaltlich **technische
Vorgaben** (Go, Fyne, sichere Credentials, Taskfile, Tests, README, CHANGELOG,
SOLID/DDD/Clean Architecture/Clean Code). Diese werden hier als **verbindliche
Anforderungen** behandelt, nicht als Ausschlüsse. Falls tatsächlich Ausschlüsse
gemeint waren: bitte melden, der Plan ändert sich dann grundlegend.

### 1.3 Geklärte Entscheidungen

| Thema | Entscheidung |
| --- | --- |
| Report-Typen v1 | Nur **DMARC Aggregate Reports (RUA)**. Schnittstellen bewusst generisch, damit RUF und TLS-RPT später ohne Umbau andocken. |
| Storage | **SQLite über `modernc.org/sqlite`** (reines Go, kein CGO → Cross-Compile bleibt einfach). |
| Mail-Zugriff v1 | **IMAP mit Benutzername/Passwort bzw. App-Passwort.** |
| Sprache | **Durchgängig Deutsch**: UI, README, CHANGELOG, Code-Kommentare, Commit-Messages. |

**Ausnahme zur Sprachregel:** Go-Bezeichner (Paket-, Typ-, Funktions-, Feldnamen)
bleiben **englisch**. Deutsche Identifier mit Umlauten brechen mit Go-Konventionen,
verschlechtern die Lesbarkeit im Zusammenspiel mit Standardbibliothek und Fyne und
erschweren spätere Mitarbeit. Alles, was Menschen lesen — Kommentare, Fehlertexte für
Nutzer, Log-Meldungen, UI-Strings, Doku — ist deutsch. Falls gewünscht, kann das
umgestellt werden; bitte kurz Bescheid geben.

---

## 2. Zielbild

Ein eigenständiges Desktop-Programm (macOS, Windows, Linux) als einzelne Binärdatei:

1. Verbindet sich mit einem konfigurierten IMAP-Postfach, in dem DMARC-Aggregate-
   Reports eingehen.
2. Holt **inkrementell** nur neue Nachrichten, entpackt die Anhänge
   (`.gz`, `.zip`, blankes `.xml`), parst das Feedback-XML.
3. Speichert die normalisierten Daten lokal in SQLite — idempotent, Duplikate werden
   erkannt und verworfen.
4. Stellt die Daten in einer Fyne-Oberfläche dar: Dashboard mit Kennzahlen und
   Diagrammen, filter-/sortier-/gruppierbare Tabellen, Detailansichten.
5. Speichert Zugangsdaten im Schlüsselbund des Betriebssystems, niemals im Klartext.

---

## 3. Technologie-Stack

| Zweck | Bibliothek | Begründung |
| --- | --- | --- |
| UI | `fyne.io/fyne/v2` | Vorgabe aus `FEATURES.md`. |
| IMAP | `github.com/emersion/go-imap/v2` | Aktueller Stand, IMAP4rev1+rev2, sauberes API. |
| MIME/Anhänge | `github.com/emersion/go-message` | Robustes Parsen von Multipart-Mails und Encodings. |
| Datenbank | `modernc.org/sqlite` (via `database/sql`) | CGO-frei, Cross-Compile ohne C-Toolchain. |
| Credentials | `github.com/zalando/go-keyring` | macOS Keychain, Windows Credential Manager, Linux Secret Service. |
| Diagramme | `github.com/wcharczuk/go-chart/v2` | Rendert nach `image.Image`, direkt in `canvas.Image` einbettbar. Hinter einem Port gekapselt (siehe 8.2). |
| Logging | `log/slog` (Standardbibliothek) | Kein zusätzliches Dependency, strukturierte Logs. |
| XML/Archive | `encoding/xml`, `compress/gzip`, `archive/zip` | Standardbibliothek reicht vollständig aus. |
| Tests | `testing` + `github.com/stretchr/testify/require` | Table-driven Tests, knappe Assertions. |
| UI-Tests | `fyne.io/fyne/v2/test` | Headless-Rendering, Widget-Interaktion ohne Display. |
| Task-Runner | `Taskfile.yml` (go-task) | Vorgabe; `task` ist lokal bereits installiert. |
| Linting | `golangci-lint` | Sammelt vet, staticcheck, errcheck, revive u. a. |

**Go-Version:** 1.27 (lokal 1.27.1 vorhanden).
**Modulpfad (Vorschlag):** `github.com/Freie-Schule/dmarc-analyzer` — siehe offene Frage O-1.

Abhängigkeiten werden bewusst knapp gehalten: alles, was die Standardbibliothek
sauber erledigt, wird nicht durch ein Dependency ersetzt.

---

## 4. Architektur

### 4.1 Schichtenmodell (Clean Architecture)

Abhängigkeiten zeigen ausschließlich **nach innen**. Die Domänenschicht kennt weder
SQL noch IMAP noch Fyne.

```
┌───────────────────────────────────────────────────────────┐
│  cmd/dmarc-analyzer  — Composition Root, Wiring, Start     │
├───────────────────────────────────────────────────────────┤
│  internal/ui         — Fyne: Views, Widgets, ViewModels    │
│  internal/infra      — IMAP, SQLite, Keyring, Charts, XML  │
├───────────────────────────────────────────────────────────┤
│  internal/app        — Use Cases (Anwendungsfälle)         │
├───────────────────────────────────────────────────────────┤
│  internal/domain     — Entities, Value Objects, Ports      │
└───────────────────────────────────────────────────────────┘
```

* **domain** — reine Fachlogik. Keine Imports außerhalb der Standardbibliothek.
  Definiert die *Ports* (Interfaces), die außen implementiert werden.
* **app** — orchestriert Use Cases, kennt nur Domänen-Ports.
* **infra** — *Adapter*: implementiert die Ports gegen konkrete Technik.
* **ui** — Fyne-Präsentation, ruft ausschließlich Use Cases auf.
* **cmd** — einziger Ort, an dem konkrete Implementierungen verdrahtet werden.

### 4.2 Bezug zu den geforderten Prinzipien

**SOLID**
* *SRP* — je Paket eine Verantwortung; Parser parst, Repository persistiert, Fetcher holt ab.
* *OCP* — neue Report-Typen (RUF, TLS-RPT) kommen als zusätzliche `ReportParser`-
  Implementierung dazu, ohne bestehenden Code zu ändern.
* *LSP* — Ports sind verhaltensdefiniert; Fakes in Tests verhalten sich wie die echten Adapter.
* *ISP* — schmale Interfaces (`ReportReader`, `ReportWriter` statt eines fetten `ReportStore`).
* *DIP* — `app` hängt an Interfaces aus `domain`, nie an `infra`.

**DDD**
* *Aggregate Root*: `AggregateReport` — Records existieren nur innerhalb eines Reports
  und werden ausschließlich über ihn geladen und gespeichert.
* *Value Objects* (unveränderlich, ohne Identität): `SourceIP`, `DomainName`,
  `DateRange`, `Disposition`, `AlignmentMode`, `PolicyEvaluation`, `AuthResults`.
* *Domain Services*: `AlignmentEvaluator` (bewertet Ausrichtung), `ReportDeduplicator`.
* *Repositories*: eines pro Aggregate, mit fachlicher Sprache
  (`FindByDomainAndPeriod`, nicht `SelectWhere`).
* *Ubiquitous Language*: Begriffe aus RFC 7489 werden 1:1 übernommen
  (Disposition, Alignment, Policy Published, Header From …) — keine Eigenerfindungen.

**Clean Code**
* Funktionen kurz und auf einer Abstraktionsebene, sprechende Namen, keine Flag-Parameter.
* Fehler werden mit `fmt.Errorf("...: %w", err)` kontextualisiert und nie verschluckt.
* Kommentare erklären das *Warum*, nicht das *Was*.
* Keine globalen Zustände außerhalb der Composition Root.

---

## 5. Projektstruktur

```
dmarc-analyzer/
├── cmd/
│   └── dmarc-analyzer/
│       ├── main.go                  # Einstiegspunkt
│       └── wire.go                  # Composition Root: Adapter verdrahten
├── internal/
│   ├── domain/
│   │   ├── report/                  # Aggregate: AggregateReport
│   │   │   ├── report.go
│   │   │   ├── record.go
│   │   │   ├── policy.go
│   │   │   ├── valueobjects.go
│   │   │   └── repository.go        # Port: ReportRepository
│   │   ├── account/                 # Aggregate: MailAccount
│   │   │   ├── account.go
│   │   │   ├── credentials.go
│   │   │   └── repository.go        # Ports: AccountRepository, CredentialStore
│   │   ├── sync/
│   │   │   ├── state.go             # SyncState je Konto/Postfach
│   │   │   └── ports.go             # Ports: MessageSource, ReportParser
│   │   └── analysis/
│   │       ├── statistics.go        # Kennzahlen, Aggregationen
│   │       └── alignment.go         # AlignmentEvaluator
│   ├── app/
│   │   ├── syncreports/             # Use Case: Reports abholen + importieren
│   │   ├── queryreports/            # Use Case: Filtern, Sortieren, Gruppieren
│   │   ├── statistics/              # Use Case: Dashboard-Kennzahlen
│   │   ├── manageaccount/           # Use Case: Konto anlegen/testen/löschen
│   │   └── exportdata/              # Use Case: CSV-/PNG-Export
│   ├── infra/
│   │   ├── imap/                    # MessageSource-Adapter
│   │   ├── dmarcxml/                # ReportParser-Adapter (XML + gz/zip)
│   │   ├── sqlite/
│   │   │   ├── migrations/          # *.sql, per go:embed eingebettet
│   │   │   ├── db.go
│   │   │   ├── migrate.go
│   │   │   ├── reportrepo.go
│   │   │   ├── accountrepo.go
│   │   │   └── syncstaterepo.go
│   │   ├── keyring/                 # CredentialStore-Adapter
│   │   ├── charts/                  # ChartRenderer-Adapter
│   │   └── config/                  # Einstellungen (nicht-geheim)
│   ├── ui/
│   │   ├── app.go                   # Fenster, Navigation, Theme
│   │   ├── dashboard/
│   │   ├── reports/
│   │   ├── sources/
│   │   ├── settings/
│   │   ├── components/              # Wiederverwendbare Widgets
│   │   └── i18n/                    # Zentrale UI-Texte (deutsch)
│   └── platform/
│       ├── logging/
│       └── paths/                   # Pfade für DB, Logs, Config
├── testdata/
│   └── reports/                     # Echte Beispiel-Reports (anonymisiert)
├── docs/
├── Taskfile.yml
├── README.md
├── CHANGELOG.md
├── FEATURES.md
├── IMPLEMENTIERUNG.md
├── go.mod
└── .golangci.yml
```

`internal/` verhindert, dass Interna versehentlich zur öffentlichen API werden.

---

## 6. Domänenmodell

### 6.1 Aggregate `AggregateReport`

Abgeleitet aus dem Schema in RFC 7489, Anhang C.

```go
// AggregateReport ist das Aggregate Root eines DMARC-Berichts.
// Records existieren nur im Kontext ihres Reports.
type AggregateReport struct {
    ID          ReportID
    Metadata    Metadata        // Absender-Org, Report-ID, Zeitraum
    Policy      PublishedPolicy // veröffentlichte DMARC-Policy
    Records     []Record        // ausgewertete Sendequellen
    ImportedAt  time.Time
    SourceRef   SourceReference // Herkunft: Konto, Postfach, UID, Dateiname
}

type Metadata struct {
    OrgName          string
    Email            string
    ExtraContactInfo string
    ReportID         string
    Range            DateRange
    Errors           []string
}

type PublishedPolicy struct {
    Domain          DomainName
    SubdomainPolicy Policy         // none | quarantine | reject
    Policy          Policy
    DKIMAlignment   AlignmentMode  // r (relaxed) | s (strict)
    SPFAlignment    AlignmentMode
    Percentage      int
    FailureOptions  string
}

type Record struct {
    SourceIP    SourceIP
    Count       int
    Evaluated   PolicyEvaluation // disposition + dkim/spf-Ergebnis + Gründe
    Identifiers Identifiers      // header_from, envelope_from, envelope_to
    Auth        AuthResults      // DKIM- und SPF-Einzelergebnisse
}
```

### 6.2 Fachliche Invarianten (in Konstruktoren erzwungen)

* Ein Report ohne `ReportID` oder ohne gültigen Zeitraum ist ungültig.
* `DateRange.Begin` liegt vor `DateRange.End`.
* `Count` je Record ist `> 0`.
* `Percentage` liegt in `[0, 100]`.
* Unbekannte Enum-Werte aus dem XML werden auf `Unknown` abgebildet, nicht verworfen —
  Provider halten sich nicht immer an den RFC.

### 6.3 Fachliche Identität und Deduplizierung

Die fachliche Identität eines Reports ist das Tripel
`(OrgName, ReportID, DateRange.Begin)`. Darauf liegt ein UNIQUE-Index. Dadurch ist der
Import auch dann idempotent, wenn dieselbe Mail doppelt abgeholt wird — etwa nach einem
`UIDVALIDITY`-Wechsel auf dem Server oder einem Wiederherstellen des Postfachs.

### 6.4 Ports (in `domain` definiert, in `infra` implementiert)

```go
// MessageSource liefert Rohnachrichten aus einer Quelle (v1: IMAP).
// Bewusst technikneutral, damit später Dateiimport oder andere Protokolle
// ohne Änderung der Use Cases andocken können.
type MessageSource interface {
    Connect(ctx context.Context, acc account.MailAccount) error
    FetchNew(ctx context.Context, state SyncState) (iter.Seq2[RawMessage, error], SyncState, error)
    Close() error
}

// ReportParser wandelt einen Rohanhang in Domänenobjekte.
// Über Supports() wird die passende Implementierung gewählt — so kommen
// RUF und TLS-RPT später additiv hinzu (Open/Closed).
type ReportParser interface {
    Supports(attachment RawAttachment) bool
    Parse(ctx context.Context, attachment RawAttachment) (*AggregateReport, error)
}

type ReportRepository interface {
    Save(ctx context.Context, r *AggregateReport) error
    Exists(ctx context.Context, key ReportKey) (bool, error)
    FindByID(ctx context.Context, id ReportID) (*AggregateReport, error)
    Query(ctx context.Context, q ReportQuery) (ReportPage, error)
}

type CredentialStore interface {
    Store(accountID string, secret []byte) error
    Retrieve(accountID string) ([]byte, error)
    Delete(accountID string) error
}
```

`FetchNew` liefert einen Iterator statt eines Slices: große Postfächer werden so
streamend verarbeitet, ohne alle Nachrichten im Speicher zu halten.

---

## 7. Kernablauf: Inkrementeller Sync

> Anforderung: „It is smart to discover new data only in the mail account."

### 7.1 Ablauf

```
Use Case SyncReports
  │
  ├─ 1. Konto laden, Passwort aus dem Schlüsselbund holen
  ├─ 2. IMAP-Verbindung aufbauen (TLS erzwungen)
  ├─ 3. Postfach auswählen, SyncState laden
  │      ├─ UIDVALIDITY stimmt überein?  → UID-basiert weiter ab LastUID+1
  │      └─ UIDVALIDITY hat gewechselt?  → vollständiger Rescan,
  │                                         Duplikate fängt der UNIQUE-Index ab
  ├─ 4. UID FETCH (BODY.PEEK[]) der neuen Nachrichten, gestreamt
  │      └─ PEEK: das \Seen-Flag bleibt unberührt, das Postfach
  │               wird nicht für andere Clients „verbraucht"
  ├─ 5. Je Nachricht:
  │      ├─ MIME zerlegen, Anhänge extrahieren
  │      ├─ Dekomprimieren (.gz / .zip / blankes .xml)
  │      ├─ Passenden ReportParser wählen und parsen
  │      ├─ Deduplizierungsschlüssel prüfen → ggf. überspringen
  │      └─ In einer Transaktion speichern (Report + alle Records)
  ├─ 6. Nach jeder erfolgreichen Nachricht SyncState fortschreiben
  │      └─ Abbruch/Absturz kostet höchstens eine Nachricht erneut
  └─ 7. Ergebnis melden: neu / übersprungen / fehlerhaft
```

### 7.2 Robustheit

* **Fehlerquarantäne** — nicht parsebare Anhänge landen mit Fehlertext in
  `failed_imports`, statt den gesamten Lauf abzubrechen. In der UI einsehbar und
  gezielt wiederholbar.
* **Rohdaten-Archiv** — das Original-XML wird optional gzip-komprimiert in
  `raw_reports` abgelegt. So kann nach einer Parser-Korrektur neu eingelesen werden,
  ohne das Postfach erneut zu befragen. Abschaltbar (Speicherplatz).
* **Kontext-Abbruch** — jeder Schritt respektiert `context.Context`; der Sync-Knopf
  in der UI kann jederzeit abgebrochen werden.
* **Backoff** — bei temporären IMAP-Fehlern exponentiell gestaffelte Wiederholung
  (3 Versuche), danach sauberer Abbruch mit verständlicher Meldung.
* **Transaktionen** — ein Report wird vollständig oder gar nicht gespeichert.

### 7.3 Nebenläufigkeit

Das Abholen (I/O-gebunden) und das Parsen (CPU-gebunden) laufen als Pipeline über
Channels mit begrenzter Worker-Anzahl (`GOMAXPROCS`, gedeckelt). Die Schreiboperationen
laufen **seriell** in einer einzigen Goroutine — SQLite mag keine konkurrierenden
Schreiber. Zusätzlich: `journal_mode=WAL`, `busy_timeout=5000`.

---

## 8. Persistenz

### 8.1 Schema (Auszug)

```sql
CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL,
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL,
    username      TEXT NOT NULL,
    mailbox       TEXT NOT NULL DEFAULT 'INBOX',
    use_tls       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL
    -- Passwort bewusst NICHT hier, sondern im Schlüsselbund
);

CREATE TABLE sync_state (
    account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    mailbox       TEXT NOT NULL,
    uid_validity  INTEGER NOT NULL,
    last_uid      INTEGER NOT NULL,
    last_sync_at  TEXT,
    PRIMARY KEY (account_id, mailbox)
);

CREATE TABLE reports (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_name      TEXT NOT NULL,
    org_email     TEXT,
    report_id     TEXT NOT NULL,
    date_begin    INTEGER NOT NULL,   -- Unix-Zeit, UTC
    date_end      INTEGER NOT NULL,
    policy_domain TEXT NOT NULL,
    policy_p      TEXT, policy_sp TEXT,
    policy_adkim  TEXT, policy_aspf TEXT,
    policy_pct    INTEGER, policy_fo TEXT,
    account_id    TEXT REFERENCES accounts(id) ON DELETE SET NULL,
    message_uid   INTEGER,
    imported_at   TEXT NOT NULL,
    UNIQUE (org_name, report_id, date_begin)
);

CREATE TABLE records (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id     INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    source_ip     TEXT NOT NULL,
    message_count INTEGER NOT NULL,
    disposition   TEXT NOT NULL,
    dkim_result   TEXT NOT NULL,      -- ausgewertet (aligned)
    spf_result    TEXT NOT NULL,
    header_from   TEXT NOT NULL,
    envelope_from TEXT,
    envelope_to   TEXT
);

CREATE TABLE record_reasons     (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 type TEXT, comment TEXT);
CREATE TABLE auth_results_dkim  (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 domain TEXT, selector TEXT, result TEXT, human_result TEXT);
CREATE TABLE auth_results_spf   (record_id INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
                                 domain TEXT, scope TEXT, result TEXT);
CREATE TABLE raw_reports        (report_id INTEGER PRIMARY KEY REFERENCES reports(id) ON DELETE CASCADE,
                                 filename TEXT, content BLOB);
CREATE TABLE failed_imports     (id INTEGER PRIMARY KEY AUTOINCREMENT, account_id TEXT, message_uid INTEGER,
                                 filename TEXT, error TEXT, raw BLOB, occurred_at TEXT NOT NULL);

CREATE INDEX idx_reports_period   ON reports(date_begin, date_end);
CREATE INDEX idx_reports_domain   ON reports(policy_domain);
CREATE INDEX idx_records_report   ON records(report_id);
CREATE INDEX idx_records_ip       ON records(source_ip);
CREATE INDEX idx_records_from     ON records(header_from);
```

Zeitstempel für Report-Zeiträume werden als **Unix-Sekunden in UTC** gespeichert —
Zeitzonenlogik gehört in die Darstellungsschicht, nicht in die Daten.

### 8.2 Weitere Persistenz-Entscheidungen

* **Migrationen** — nummerierte `.sql`-Dateien in `internal/infra/sqlite/migrations/`,
  per `go:embed` in die Binärdatei eingebettet, angewendet durch einen kleinen eigenen
  Migrator (~60 Zeilen) mit Tabelle `schema_migrations`. Kein zusätzliches Dependency.
* **PRAGMAs beim Öffnen** — `journal_mode=WAL`, `foreign_keys=ON`,
  `busy_timeout=5000`, `synchronous=NORMAL`.
* **Performance** — Batch-Inserts von Records über Prepared Statements innerhalb
  einer Transaktion. `modernc.org/sqlite` ist langsamer als die CGO-Variante; bei
  typischen Report-Größen (einige hundert Records) ist das irrelevant, beim Erstimport
  großer Archive macht Batching den Unterschied.
* **Speicherort** — `os.UserConfigDir()/dmarc-analyzer/dmarc.db`, plattformkonform.
* **ChartRenderer als Port** — `analysis.ChartRenderer` liefert `image.Image`. v1
  implementiert das mit `go-chart`. Sollten sich später native, interaktive
  Fyne-Widgets (Hover, Klick auf Datenpunkt) lohnen, wird nur der Adapter getauscht.

---

## 9. Sicherer Umgang mit Zugangsdaten

> Anforderung: „The credentials are securely stored."

| Regel | Umsetzung |
| --- | --- |
| Passwort nie in der DB | Nur `accounts`-Metadaten in SQLite; Secret im OS-Schlüsselbund unter Service `de.freie-schule.dmarc-analyzer`, Key = Account-ID. |
| Passwort nie im Log | Eigener Typ `Secret` mit `String()`/`MarshalJSON()`, die `"***"` zurückgeben — Leaks über `%v` oder JSON-Dumps sind damit strukturell ausgeschlossen. |
| Transport verschlüsselt | IMAPS (Port 993) als Standard; STARTTLS als Alternative. Klartext-IMAP ist nur mit expliziter Bestätigung möglich und wird in der UI gewarnt. |
| Zertifikate | Reguläre Prüfung gegen den System-Trust-Store. `InsecureSkipVerify` existiert nicht als Option. |
| Kurze Lebensdauer im Speicher | Secret wird erst unmittelbar vor dem Login gelesen und danach mit `Zero()` überschrieben. |
| Linux-Fallback | Ohne Secret Service (D-Bus) greift ein verschlüsselter Dateispeicher: AES-256-GCM, Schlüssel aus einer Master-Passphrase via `scrypt`. Abfrage beim Programmstart. |
| Löschen heißt löschen | Kontolöschung entfernt Metadaten **und** Schlüsselbundeintrag. |

---

## 10. UI-Konzept (Fyne)

### 10.1 Navigation

Haupt­fenster mit seitlicher Navigation (`container.NewBorder` + `widget.List`):

| Ansicht | Inhalt |
| --- | --- |
| **Übersicht** | Kennzahlen-Kacheln, Zeitreihe, Top-Absender, Verteilung der Dispositions. |
| **Berichte** | Virtualisierte Tabelle aller Reports; Filter nach Zeitraum, Domain, Absender-Org. Doppelklick öffnet Details. |
| **Bericht-Detail** | Metadaten, veröffentlichte Policy, Record-Tabelle mit Auth-Ergebnissen. |
| **Sendequellen** | Aggregiert nach Quell-IP: Volumen, Pass-Rate, PTR/rDNS, erkannter Dienst. |
| **Einstellungen** | Konten verwalten, Verbindung testen, Sync-Intervall, Aufbewahrungsdauer, Theme. |
| **Protokoll** | Sync-Historie, fehlgeschlagene Importe mit Wiederholen-Aktion. |

### 10.2 Kennzahlen auf der Übersicht

* Gesamtzahl ausgewerteter Nachrichten im gewählten Zeitraum
* DMARC-Pass-Rate (Anteil der Nachrichten mit `dkim=pass` **oder** `spf=pass` nach Alignment)
* SPF-Alignment-Rate und DKIM-Alignment-Rate getrennt
* Anzahl unterschiedlicher Sendequellen
* Volumen nach Disposition: `none` / `quarantine` / `reject`
* Veränderung gegenüber der Vorperiode (Trendpfeil)

### 10.3 Visualisierungen

* **Zeitreihe** — Nachrichtenvolumen pro Tag, gestapelt nach Pass/Fail.
* **Balken** — Top-10-Sendequellen nach Volumen, eingefärbt nach Pass-Rate.
* **Donut** — Verteilung der Dispositions.
* **Heatmap** — Sendequelle × Tag, Farbe = Pass-Rate. Macht ausfallende Dienste sofort sichtbar.
* **Tabelle mit Gruppierung** — nach Domain, Org oder Quell-IP klappbar.

### 10.4 Fyne-Praxis

* **Nie im UI-Thread blockieren** — jede I/O-Operation läuft in einer Goroutine;
  Aktualisierungen gehen über `fyne.Do()` zurück in den UI-Thread.
* **`widget.Table` mit Lazy-Datenquelle** — die Tabelle virtualisiert bereits das
  Rendern; die Datenquelle muss seitenweise aus SQLite nachladen (`LIMIT`/`OFFSET`
  bzw. Keyset-Pagination), damit auch 100.000 Records flüssig bleiben.
* **`data binding`** für einfache Felder, eigene ViewModels für komplexe Ansichten.
* **Eigenes Theme** mit Farbpalette, die sowohl im hellen als auch im dunklen Modus
  funktioniert; Pass/Fail-Farben zusätzlich durch Symbole unterscheidbar (Barrierefreiheit).
* **Fortschrittsanzeige** beim Sync mit Abbruch-Möglichkeit.
* **Desktop-Benachrichtigung** (`fyne.App.SendNotification`) nach Abschluss eines
  Hintergrund-Syncs.

---

## 11. Vorschläge für zusätzliche Funktionen

> Zur Anforderung „Propose any features I might have forgotten."

### Hoher Nutzen, geringer Aufwand

1. **Datei-/Ordner-Import** — `.eml`, `.zip`, `.xml` per Drag & Drop einlesen. Schon
   für die Entwicklung unverzichtbar, ermöglicht Offline-Betrieb und Migration von
   Altbeständen. Fällt fast nebenbei ab, weil `MessageSource` bereits abstrahiert ist.
2. **rDNS-/PTR-Auflösung** der Quell-IPs mit Cache. Aus „192.0.2.45" wird
   „mail-out.mailchimp.com" — der Unterschied zwischen Rohdaten und Erkenntnis.
3. **Erkennung bekannter Dienste** — kuratierte Liste (Google Workspace, Microsoft 365,
   Mailchimp, SendGrid, Brevo, Postmark …), Abgleich über PTR und IP-Bereiche. Macht
   sofort sichtbar, welcher legitime Dienst noch nicht korrekt authentifiziert ist.
4. **Export** — gefilterte Ansicht als CSV, Diagramme als PNG.
5. **DNS-Prüfung der eigenen Domain** — aktuellen `_dmarc`-TXT-Record, SPF und
   DKIM-Selektoren live abfragen und die Syntax validieren. Antwortet auf die Frage
   „stimmt meine Konfiguration überhaupt?", die aus Reports allein nie hervorgeht.

### Mittlerer Aufwand, hoher fachlicher Wert

6. **Diagnose-Assistent** — zu einer fehlschlagenden Quelle in Klartext erklären,
   *warum* sie fehlschlägt (SPF fehlt / SPF nicht ausgerichtet / DKIM-Signatur ungültig /
   Weiterleitung ohne SRS) und was zu tun ist.
7. **Policy-Reifegrad** — bewerten, ob ein Wechsel von `p=none` nach `quarantine` oder
   `reject` gefahrlos möglich ist: „98,7 % der letzten 30 Tage bestehen DMARC; 2 nicht
   klassifizierte Quellen offen."
8. **Alarmierung** — Desktop-Benachrichtigung bei Pass-Raten-Einbruch, neuer unbekannter
   Sendequelle oder ausbleibenden Reports.
9. **Zeitraumvergleich** — zwei Perioden nebeneinander, Differenzen hervorgehoben.
10. **SPF-Lookup-Zähler** — Warnung beim Überschreiten des 10-DNS-Lookup-Limits (RFC 7208),
    eine der häufigsten stillen Fehlerursachen.

### Betrieb und Datenschutz

11. **Aufbewahrungsrichtlinie** — Reports älter als *n* Monate automatisch löschen.
    DSGVO-relevant, weil Aggregate Reports IP-Adressen enthalten.
12. **Backup/Restore** — DB sichern und wiederherstellen (`VACUUM INTO`).
13. **Hintergrund-Sync per Zeitplan** — z. B. stündlich, mit Statusanzeige.
14. **Headless-CLI-Modus** — `dmarc-analyzer sync --headless` für Cron/launchd,
    ohne UI-Start. Aus der Clean Architecture ergibt sich das fast kostenlos, weil die
    Use Cases UI-unabhängig sind.
15. **Mehrere Konten und Domains** parallel, mit Domain-Filter in allen Ansichten.

### Später (Schnittstellen sind vorbereitet)

16. DMARC Forensic Reports (RUF) — mit Hinweis auf personenbezogene Inhalte.
17. TLS-RPT (RFC 8460).
18. BIMI-Bereitschaftsprüfung.
19. MTA-STS-Policy-Prüfung.

**Empfehlung für v1:** Punkte 1–4 und 11 mit einplanen; 5–8 als v1.1. Die übrigen sind
begründete Optionen, kein Muss.

---

## 12. Teststrategie

> Anforderung: „write tests."

### 12.1 Verteilung

```
        ╱╲        wenige End-to-End-Tests (Sync gegen IMAP-Fake)
       ╱  ╲       Integrationstests (SQLite, Parser mit echten Fixtures)
      ╱____╲      viele Unit-Tests (Domäne, Use Cases)
```

### 12.2 Je Schicht

| Schicht | Vorgehen | Zielabdeckung |
| --- | --- | --- |
| `domain` | Reine Unit-Tests, table-driven, keine Mocks nötig — die Schicht hat keine Abhängigkeiten. | ≥ 90 % |
| `app` | Use Cases gegen **Fakes** der Ports (handgeschrieben, kein Mock-Framework). | ≥ 85 % |
| `infra/dmarcxml` | **Golden-File-Tests** gegen echte, anonymisierte Reports von Google, Microsoft, Yahoo, Mail.ru, Enterprise-Anbietern — jeder Provider weicht anders vom RFC ab. Zusätzlich Fuzzing auf dem XML-Parser. | ≥ 85 % |
| `infra/sqlite` | Integrationstests gegen eine temporäre Datei-DB (nicht `:memory:`, damit WAL und Transaktionen realistisch getestet werden). Migrationen vorwärts und wiederholt anwenden. | ≥ 80 % |
| `infra/imap` | Tests gegen einen In-Process-IMAP-Server (`go-imap`-Serverkomponente) mit vorbereiteten Nachrichten. Prüft besonders UID-Logik und `UIDVALIDITY`-Wechsel. | ≥ 70 % |
| `ui` | `fyne.io/fyne/v2/test`: Widget-Aufbau, Navigation, Formularvalidierung. Keine Pixelvergleiche — die sind zu brüchig. | Smoke-Level |

### 12.3 Besondere Testfälle

* Report mit 0 Records; Report mit 10.000 Records.
* `UIDVALIDITY`-Wechsel mitten im Sync → vollständiger Rescan ohne Duplikate.
* Doppelt zugestellte identische Mail → genau ein Datensatz.
* Beschädigtes Gzip, Zip mit mehreren Dateien, Zip-Bombe (Größenlimit beim Entpacken).
* XML mit unbekannten Elementen, fehlendem `pct`, nicht-RFC-konformen Enum-Werten.
* XXE-Schutz: externe Entities werden abgelehnt (Go's `encoding/xml` löst sie nicht
  auf — wird durch einen Test festgeschrieben, damit das so bleibt).
* Abbruch per `context.Cancel` mitten im Sync → konsistenter Zustand.

### 12.4 Werkzeuge

* `go test -race` in jedem Lauf — bei einer Goroutine-Pipeline nicht verhandelbar.
* `task cover` erzeugt einen HTML-Coverage-Report.
* Testfixtures liegen in `testdata/`; **alle Domains und IPs werden anonymisiert**
  (`example.com`, RFC-5737-Adressbereiche).

---

## 13. Taskfile

> Anforderung: „generate a Taskfile with the common tasks."

| Task | Zweck |
| --- | --- |
| `task setup` | Werkzeuge installieren (`golangci-lint`, `fyne` CLI), `go mod download`. |
| `task build` | Binärdatei nach `bin/` bauen, Version per `-ldflags` einbetten. |
| `task run` | Programm im Entwicklungsmodus starten. |
| `task test` | Alle Tests mit `-race`. |
| `task test:unit` | Nur schnelle Tests (ohne Build-Tag `integration`). |
| `task test:integration` | Integrationstests (Build-Tag `integration`). |
| `task cover` | Coverage messen und als HTML öffnen. |
| `task lint` | `golangci-lint run`. |
| `task fmt` | `gofmt -s -w` + `goimports`. |
| `task tidy` | `go mod tidy` und Prüfung auf ungenutzte Abhängigkeiten. |
| `task package:darwin` | `.app`-Bundle via `fyne package`. |
| `task package:windows` | `.exe` mit Icon. |
| `task package:linux` | `.tar.xz` mit Desktop-Eintrag. |
| `task release` | Alle Plattformen bauen, Checksummen erzeugen. |
| `task clean` | Build-Artefakte entfernen. |
| `task check` | `fmt` + `lint` + `test` — das, was auch die CI ausführt. |

`task check` ist der Standardbefehl vor jedem Commit.

---

## 14. Dokumentation

**README.md** (deutsch): Was das Programm tut, Screenshots, Installation je Plattform,
Einrichtung des IMAP-Kontos (inklusive Hinweis zu App-Passwörtern), Erklärung der
Kennzahlen, Speicherorte von DB und Konfiguration, Datenschutzhinweis, Entwicklungs-
Setup, Lizenz.

**CHANGELOG.md**: Format nach *Keep a Changelog*, Versionierung nach *SemVer*.
Abschnitte `Hinzugefügt` / `Geändert` / `Behoben` / `Entfernt` / `Sicherheit`. Wird bei
jedem Merge gepflegt, nicht erst beim Release.

**docs/**: Architekturüberblick mit Diagramm, ADRs (Architecture Decision Records) für
die tragenden Entscheidungen — je ein kurzes Dokument zu SQLite-Wahl, CGO-Freiheit,
Keyring-Strategie und Chart-Rendering. Der Wert liegt darin, in einem Jahr noch zu
wissen, *warum* etwas so ist.

---

## 15. Phasenplan

| Phase | Inhalt | Ergebnis |
| --- | --- | --- |
| **0 — Grundgerüst** | `go mod init`, Verzeichnisstruktur, `Taskfile.yml`, `.golangci.yml`, GitHub-Actions-Workflow, README-/CHANGELOG-Rohfassung, Logging, Pfad-Auflösung. | `task check` läuft grün auf leerem Projekt. |
| **1 — Domäne & Parser** | Entities, Value Objects, Invarianten, Ports. XML-Parser inklusive gzip/zip-Entpacken. Fixtures mehrerer Provider. | Beliebiger Report wird korrekt zu Domänenobjekten; Golden-Tests grün. |
| **2 — Persistenz** | SQLite-Anbindung, Migrator, Repositories, Deduplizierung, Query-/Aggregations-Funktionen. | Reports werden gespeichert und wieder gelesen; Integrationstests grün. |
| **3 — Mail & Sicherheit** | IMAP-Adapter, Keyring-Adapter, `SyncState`, inkrementelle Logik, Fehlerquarantäne. | `SyncReports` holt aus einem echten Postfach nur Neues. |
| **4 — Anwendungsschicht** | Use Cases vollständig, Statistiken, Export, CLI-Modus `sync --headless`. | Kompletter Ablauf ohne UI nutzbar und testbar. |
| **5 — UI-Grundgerüst** | Fenster, Navigation, Theme, Einstellungen mit Verbindungstest, Berichtstabelle, Detailansicht, Sync mit Fortschritt. | Bedienbares Programm für den Kern-Use-Case. |
| **6 — Auswertung** | Dashboard, Diagramme, Sendequellen-Ansicht, rDNS, Dienst-Erkennung, Filter und Gruppierung. | Die in `FEATURES.md` geforderte Visualisierung steht. |
| **7 — Feinschliff & Release** | Datei-Import, Aufbewahrungsrichtlinie, Benachrichtigungen, Hintergrund-Sync, Barrierefreiheit, Performance mit großen Datenmengen, Packaging für drei Plattformen, Doku vervollständigen. | Version 1.0.0. |

Jede Phase endet mit grünem `task check` und einem gepflegten CHANGELOG-Eintrag.
Die Phasen 1–4 sind vollständig ohne UI testbar — das ist der eigentliche Gewinn der
gewählten Architektur und hält die Rückkopplungsschleife kurz.

---

## 16. Risiken und Gegenmaßnahmen

| Risiko | Auswirkung | Gegenmaßnahme |
| --- | --- | --- |
| `go-imap/v2` ist noch in aktiver Entwicklung, API kann sich ändern | Anpassungsaufwand bei Updates | Nur der Adapter hinter `MessageSource` ist betroffen; Version in `go.mod` gepinnt. |
| Provider liefern RFC-abweichendes XML | Import schlägt fehl | Toleranter Parser, `Unknown`-Enums, Fehlerquarantäne statt Abbruch, Rohdaten-Archiv für späteres Neueinlesen. |
| `modernc.org/sqlite` langsamer als CGO-Variante | Träger Erstimport | Batch-Inserts in Transaktionen, Indizes erst nach dem Massenimport, Benchmarks in Phase 2. |
| Fyne-Tabellen bei sehr vielen Zeilen | UI ruckelt | Keyset-Pagination in der Datenquelle, Aggregationen in SQL statt in Go. |
| Linux ohne Secret Service | Kein Schlüsselbund verfügbar | Verschlüsselter Dateispeicher als Fallback (siehe Abschnitt 9). |
| Zip-Bombe im Anhang | Speicher erschöpft | Harte Obergrenze für entpackte Größe (z. B. 100 MB), `io.LimitReader`. |
| Aggregate Reports enthalten IP-Adressen | DSGVO-Pflichten | Rein lokale Verarbeitung, keine Cloud-Übertragung, Aufbewahrungsrichtlinie, Hinweis im README. |
| Umfang wächst über v1 hinaus | Verzögerung | Abschnitt 11 priorisiert bewusst; alles jenseits v1 bleibt bis nach 1.0.0 liegen. |

---

## 17. Offene Fragen

| Nr. | Frage | Vorschlag, falls keine Antwort |
| --- | --- | --- |
| **O-1** | Wie lautet der Modulpfad / die Repo-URL? | `github.com/Freie-Schule/dmarc-analyzer` |
| **O-2** | Sollen Go-Bezeichner wirklich englisch bleiben (Kommentare und UI deutsch)? Siehe 1.3. | Ja, englische Bezeichner. |
| **O-3** | Welche Lizenz? | MIT |
| **O-4** | Sollen verarbeitete Mails im Postfach markiert oder verschoben werden, oder unberührt bleiben? | Unberührt (`BODY.PEEK`), da mehrere Clients dasselbe Postfach lesen könnten. Optional abschaltbar. |
| **O-5** | Gibt es Beispiel-Reports aus dem echten Postfach für die Fixtures? | Sonst werden aus öffentlichen RFC-Beispielen synthetische Fixtures erzeugt. |
| **O-6** | Sind Windows und Linux tatsächlich Zielplattformen, oder reicht macOS? | Alle drei bauen, getestet wird primär auf macOS. |
| **O-7** | Standard-Aufbewahrungsdauer für Reports? | 24 Monate, in den Einstellungen änderbar. |
| **O-8** | Soll es einen Headless-/CLI-Modus geben (Vorschlag 14)? | Ja, in Phase 4 — der Aufwand ist gering, der Nutzen für automatisierte Läufe hoch. |
