# AGENTS.md

Leitfaden für KI-Coding-Agenten in diesem Repository. Menschliche
Mitwirkende finden dieselben Informationen ausführlicher in
[`IMPLEMENTIERUNG.md`](IMPLEMENTIERUNG.md) (Architektur) und
[`UMSETZUNGSPLAN.md`](UMSETZUNGSPLAN.md) (Arbeitsplan, Fortschritt).

## Worum es geht

DMARC Analyzer: ein Go/Fyne-Desktopprogramm, das DMARC-Aggregate-Reports aus
einem IMAP-Postfach holt, lokal in SQLite speichert und auswertet. Aktueller
Stand und nächste Schritte immer zuerst in `UMSETZUNGSPLAN.md` prüfen, bevor
mit einer Änderung begonnen wird — dort steht, welches Arbeitspaket (AP)
gerade dran ist und was bereits abgehakt ist.

## Sprachregel — wichtig, bricht sonst CI

- **Kommentare, Fehlertexte, Log-Meldungen, UI-Strings, Doku: Deutsch.**
- **Go-Bezeichner (Pakete, Typen, Funktionen, Felder): Englisch.**
- Deshalb ist `misspell` im Linter deaktiviert (`.golangci.yml`) — ein
  englisches Wörterbuch würde bei jedem deutschen Kommentar Fehlalarm
  schlagen. Nicht wieder aktivieren, ohne dieses Problem zu lösen.

## Befehle

Alles läuft über [Task](https://taskfile.dev/), nicht direkt über `go`:

```sh
task check        # fmt + lint + test — vor jedem Commit, das hier auch die CI ausführt
task test          # go test ./... -race
task test:unit      # nur schnelle Tests (-short, ohne Zip-/Gzip-Bomben-Tests)
task lint           # golangci-lint run
task build           # Binärdatei nach bin/
task --list          # alle Tasks
```

`task check` **muss grün sein**, bevor eine Änderung als fertig gilt.

## Architektur — Abhängigkeitsrichtung ist bindend

Clean Architecture, vier Schichten, Abhängigkeiten zeigen nur nach innen:

```
cmd/dmarc-analyzer  →  internal/ui  →  internal/app  →  internal/domain
                        internal/infra ─────────────────↗
```

- `internal/domain/*`: reine Fachlogik, **keine** Imports außerhalb der
  Standardbibliothek. Kein `encoding/xml`-Import hier, kein SQL, kein Fyne.
- `internal/app/*`: Use Cases, hängen nur an domain-Ports (Interfaces).
- `internal/infra/*`: Adapter, implementieren die domain-Ports gegen
  konkrete Technik (SQLite, IMAP, Keyring, go-chart, dmarcxml).
- `internal/ui/*`: Fyne, ruft ausschließlich Use Cases aus `internal/app` auf.
- `cmd/dmarc-analyzer`: einziger Ort, an dem Adapter mit Use Cases verdrahtet
  werden (Composition Root).

Details und Begründung (SOLID/DDD/Clean Code) in `IMPLEMENTIERUNG.md`
Abschnitt 4.

## Namenskonvention: kein Package-Stutter

Domänentypen heißen nicht `report.ReportKey`, sondern `report.Key` — sonst
stuttert der qualifizierte Name (`report.ReportKey`). Ausnahme: wenn die
Kurzform mit einem Feldnamen kollidieren würde (`report.ReportID` bleibt so,
weil `AggregateReport.ID ID` unlesbar wäre — ebenso `account.AccountID`
wegen `MailAccount.ID ID` — siehe Kommentar im jeweiligen Code).
`golangci-lint` (revive `exported`-Regel) erzwingt das; nicht mit
`//nolint` pauschal umgehen, sondern umbenennen, außer eine echte Kollision
verhindert es.

## SQLite: IN-Klausel mit vielen Werten vermeiden

Eine `WHERE x IN (?, ?, ..., ?)`-Klausel mit einem Platzhalter je Zeile
skaliert schlecht — bei 10.000 Werten dauerte allein das Vorbereiten des
Statements >3s (gemessen in `internal/infra/sqlite`, siehe
`reportrecords.go`). Wenn der Filterwert (hier: `report_id`) bereits
bekannt ist, stattdessen über die Fremdschlüsselbeziehung joinen
(`JOIN records ON records.id = child.record_id WHERE records.report_id = ?`)
statt vorher alle IDs zu laden und darüber eine IN-Klausel zu bauen. Bei
neuen Batch-Ladefunktionen in `internal/infra/sqlite` immer mit
realistisch großen Datenmengen testen (siehe `reportperf_test.go`), nicht
nur mit einer Handvoll Testzeilen — dort fällt das Problem nicht auf.

## Abhängigkeiten

**Nichts in `go.mod` aufnehmen, das nicht tatsächlich importiert wird.**
`go.mod` wächst organisch pro Arbeitspaket, wenn Code die jeweilige
Bibliothek wirklich importiert — nicht vorab. Recherchierte Zielversionen
für später gebrauchte Bibliotheken (Fyne, go-imap, go-message, sqlite,
go-keyring, go-chart) stehen in `docs/DEPENDENCIES.md`, nicht in `go.mod`.
`task tidy` entfernt ungenutzte Requires ohnehin wieder.

## Tests

- `github.com/stretchr/testify/require` für Assertions, keine eigenen
  `if err != nil { t.Fatalf(...) }`-Ketten.
- Table-driven mit `t.Run(...)`, wo mehrere Fälle dieselbe Logik prüfen.
- `t.Parallel()`, außer der Test nutzt `t.Setenv()` (inkompatibel).
- Langsame Tests (> ~1 s, z. B. Zip-/Gzip-Bomben mit >100 MB Testdaten) mit
  `if testing.Short() { t.Skip(...) }` markieren, damit `task test:unit`
  schnell bleibt.
- Fixtures für den DMARC-Parser: `testdata/reports/<stil>/...`, IPs aus dem
  RFC-5737-Bereich (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`),
  Domains als `example.com`/`example.org` — nie echte Adressen oder Domains.
- Coverage-Zielwerte je Schicht stehen in `IMPLEMENTIERUNG.md` Abschnitt 12.2
  (domain ≥ 90 %, app ≥ 85 %, dmarcxml/sqlite/imap gestaffelt).

## Sonst noch wichtig

- **Nicht selbstständig committen.** Änderungen fertigstellen, `git status`/
  `git diff` zeigen, aber `git commit` nur auf ausdrückliche Anweisung.
- RFC 7489 ist die fachliche Referenz für alles rund um DMARC-Aggregate-
  Reports (Feldnamen, Enum-Werte, Defaults wie `pct=100` oder
  `adkim/aspf=r`, wenn das Feld fehlt).
- Unbekannte/RFC-abweichende Enum-Werte werden nie verworfen, sondern auf
  einen `Unknown`-Wert abgebildet — Provider halten sich nicht immer an den
  RFC, der Import darf daran nicht scheitern.

## Maskierte Typen (Secret & Co.): `%#v` nicht vergessen

`String()`/`MarshalJSON()` reichen nicht, um einen Wert wirklich
unauslesbar zu machen — `%#v` benutzt `fmt.GoStringer` (`GoString()`),
nicht `fmt.Stringer`. Ohne eigene `GoString()`-Methode zeigt `%#v` bei
einem Struct mit einem `[]byte`-Feld die Rohbytes hex-kodiert (trivial
rückführbar), obwohl `%v`/`%+v` bereits korrekt maskiert sind — gefunden in
`internal/domain/account.Secret` (siehe `secret.go`/`secret_test.go`). Bei
jedem neuen maskierten Typ: `GoString()` mit ergänzen, und im Test nicht
nur auf den Klartext-Substring prüfen, sondern zusätzlich auf die
hex-kodierte Form (`encoding/hex.EncodeToString`) — sonst fällt genau diese
Lücke im Test nicht auf.

## go-imap/v2 ist Beta — bekannte Fallstricke

- `imapmemserver.User.Append(mailbox, reader, nil)` **panickt** (Nil-Pointer,
  `mailbox.go: appendBytes` liest `options.Time`/`options.Flags` ungeprüft).
  Immer `&imap.AppendOptions{}` statt `nil` übergeben. Siehe
  `internal/infra/imap/testserver_test.go`.
- Die `Dial*`-Funktionen in `imapclient` sind nicht context-fähig. Eigene
  Verbindung per `net.Dialer`/`tls.Dialer` mit `DialContext` aufbauen und an
  `imapclient.New(conn, opts)` übergeben (siehe `dial.go`); blockierende
  Befehle (`Login().Wait()`, `Fetch()`/`Next()`) über `runCtx()` context-fähig
  machen (schließt die Verbindung bei `ctx.Done()`, das lässt den
  blockierenden Aufruf mit einem Fehler zurückkehren).
- Bei jedem `go get` innerhalb dieses Moduls: `go mod tidy` nicht vergessen —
  sonst bleiben frisch direkt importierte Pakete fälschlich als
  `// indirect` markiert (passiert, wenn `go get` mehrere transitive
  Abhängigkeiten in einem Rutsch auflöst).

## Fakes für `account.CredentialStore`: defensiv kopieren

`CredentialStore.Store` hat einen impliziten, jetzt am Port dokumentierten
Vertrag: Implementierungen dürfen sich nach Rückkehr nicht mehr auf
`secret.Expose()` beziehen, weil Aufrufer das übergebene `Secret` direkt
danach mit `Zero()` überschreiben dürfen. Die echten Adapter (`OSStore`,
`FileStore`) erfüllen das automatisch (String-Konversion bzw.
Verschlüsselung verbrauchen die Bytes synchron). Ein Test-Fake, der das
`Secret` nur flach speichert (`f.secrets[id] = s`), teilt das
Backing-Array mit dem Original — ein späteres `Zero()` beim Aufrufer leert
dann auch den "gespeicherten" Wert. Immer `account.NewSecret(s.Expose())`
statt `s` speichern. Gefunden über einen echten Testausfall in
`cmd/dmarc-analyzer/cmd_account_test.go` (AP 4).
