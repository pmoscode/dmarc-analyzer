# AGENTS.md

Leitfaden für KI-Coding-Agenten in diesem Repository. Menschliche
Mitwirkende finden dieselben Informationen ausführlicher in
[`docs/architecture.md`](docs/architecture.md) (Architektur) und
[`docs/features/`](docs/features/) (Feature-Dokumentation pro Domäne).
`docs/archive/` enthält die Planungsdokumente der früheren
Desktop-Ära — historischer Kontext, kein aktueller Stand mehr (siehe
`docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`).

## Worum es geht

DMARC Analyzer: ein Go-Programm, das als Docker-Container läuft, DMARC-
Aggregate-Reports aus einem IMAP-Postfach holt, in SQLite speichert und
über eine eingebettete Web-Oberfläche auswertet. Komplett über
Umgebungsvariablen konfiguriert (`internal/infra/envconfig`, siehe
`docs/features/deployment.md`), Zugriffsschutz per OIDC gegen einen
Authentik-IdP (`docs/features/auth.md`). Genau ein IMAP-Konto pro
Container, kein Konto-CRUD in der Oberfläche.

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
cmd/dmarc-analyzer  →  internal/web  →  internal/app  →  internal/domain
                        internal/infra ─────────────────↗
```

- `internal/domain/*`: reine Fachlogik, **keine** Imports außerhalb der
  Standardbibliothek. Kein `encoding/xml`-Import hier, kein SQL.
- `internal/app/*`: Use Cases, hängen nur an domain-Ports (Interfaces).
- `internal/infra/*`: Adapter, implementieren die domain-Ports gegen
  konkrete Technik (SQLite, IMAP, dmarcxml, `envconfig` für ENV-Werte).
- `internal/web/*`: Go-`html/template` + Chart.js (`static/charts.js`),
  ruft ausschließlich Use Cases aus `internal/app` auf. Diagrammlogik
  (Aggregation, Filter, Drill-down-Ziele) gehört nach Go — `charts.js`
  bekommt fertig aufbereitete JSON-Daten und bleibt bewusst dünn. Enthält
  außerdem den OIDC-Login-Flow (`auth.go`/`oidc.go`) — bewusst hier und
  nicht in `internal/app`, weil Sitzungen/Cookies reine
  Web-Ausliefer-Belange sind, keine Fachlogik.
- `cmd/dmarc-analyzer`: einziger Ort, an dem Adapter mit Use Cases verdrahtet
  werden (Composition Root) — liest hier auch die ENV-Konfiguration
  (`envconfig.Load()`).

Details und Begründung (SOLID/DDD/Clean Code) in `docs/architecture.md`.

## Web-Oberfläche: keine Inline-Skripte, jede Zustandsänderung per POST mit CSRF

- Content-Security-Policy verbietet `unsafe-inline` (siehe
  `internal/web/middleware.go`, `middleware_test.go`) — jedes Verhalten
  gehört in `internal/web/static/*.js`, nie in ein `<script>`-Tag oder ein
  `onclick`-Attribut im Template.
- Jede Zustandsänderung (Formular, Knopf mit Seiteneffekt) ist ein POST mit
  CSRF-Token (`templateFuncs["csrfToken"]`, geprüft von `requireCSRF`) —
  niemals ein GET mit Seiteneffekt.
- Jede neue Seite/jeder neue Template-Zustand (leer, Fehler, gefüllt)
  gehört mit `httptest` gerendert und als HTML geparst getestet — ein
  Template-Fehler soll im Test auffallen, nicht erst im Browser.

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
`go.mod` wächst organisch, wenn Code eine Bibliothek wirklich importiert —
nicht vorab. Konkret gepinnte Versionen stehen in `docs/DEPENDENCIES.md`,
nicht in `go.mod`. `task tidy` entfernt ungenutzte Requires ohnehin wieder.

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
- `internal/web`-Tests gegen OIDC-geschützte Routen brauchen einen echten
  Server: `New(ctx, deps, opts)` macht beim Start eine echte
  OIDC-Discovery-Anfrage gegen `opts.OIDC.IssuerURL`. Dafür gibt es
  `internal/web/fakeoidc_test.go` (`newFakeOIDCProvider`) — ein
  minimaler, lokaler Identity-Provider (Discovery/JWKS/Authorize/Token,
  echt signierte ID-Tokens), kein Mock der Client-Logik. Siehe
  `server_test.go` (`newTestServer`, `loginViaFakeOIDC`,
  `beginFakeOIDCLogin`) für das Muster.

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

## Sitzungs-Cookies in Tests: `net/http/cookiejar` verwirft Secure-Cookies auf `http://`-Testservern

`auth.go` setzt das Sitzungs-Cookie mit `Secure: true` (Produktionsbetrieb
läuft hinter einem TLS-terminierenden Reverse-Proxy). `net/http/cookiejar`
setzt RFC 6265 korrekt um: ein per `SetCookies` gespeichertes
Secure-Cookie wird bei einem späteren `Cookies(u)`-Aufruf für eine
`http://`-URL **stillschweigend nicht zurückgegeben** — ein mit
`http.Client{Jar: cookiejar.New(nil)}` gebauter Testclient verliert die
Sitzung dadurch nach dem ersten Redirect, ohne dass ein Fehler auftritt
(einfach ein 303 zu `/anmelden` statt der erwarteten Seite). Zwei
Lösungen, je nach Testart: `internal/web/api_charts_test.go`
(`sessionTransport`) umgeht das Cookie-Handling komplett und injiziert das
Cookie über einen eigenen `http.RoundTripper` — für Handler-Tests, die
keine echte Anmeldung durchlaufen (`authenticatedClient`). Für Tests, die
den echten OIDC-Redirect-Tanz durchlaufen müssen (`server_test.go`,
`testCookieJar`), ein bewusst vereinfachter `http.CookieJar`, der
Secure/Domain/Path ignoriert. Beide Muster nicht durch einen
gewöhnlichen `cookiejar.New(nil)` ersetzen.

## Diagrammfarben (`internal/web/static/app.css`) folgen der `dataviz`-Skill-Referenzpalette

Die konkreten Hex-Werte in `:root`/`prefers-color-scheme: dark`
(Primärblau `#2a78d6`/`#3987e5`, Status-Grün/-Gelb/-Rot
`#0ca30c`/`#fab219`/`#d03b3b`) stammen unverändert aus der
Referenzpalette der `dataviz`-Skill (`references/palette.md`) — **nicht**
frei erfunden. Dieselben Statusfarben verwendet `internal/web/static/
charts.js` (Pass/Fail/Disposition-Diagramme) über dieselben CSS-Variablen
(`cssVar("--status-good")` usw.) — Oberfläche und Diagramme sprechen
dadurch dieselbe Farbsprache. Vor einer Änderung dieser Werte:
`dataviz`-Skill laden und `scripts/validate_palette.js` gegen die neuen
Werte laufen lassen (nicht nach Auge entscheiden) — siehe dortige
Anleitung. Statusfarben sind laut Skill bewusst **modusunabhängig fest**
(nicht pro hell/dunkel verschieden), Primärfarbe/Oberflächen dagegen schon.

## Chart.js `responsive: true` + `maintainAspectRatio: false`: Canvas braucht einen Elternknoten mit fester Höhe

Chart.js beobachtet im responsiven Modus per `ResizeObserver` den
**direkten Elternknoten** des `<canvas>`, um dessen Höhe/Breite
anzupassen. Hat dieser Elternknoten selbst keine explizite, vom Canvas
unabhängige Höhe (z. B. weil er wie eine gewöhnliche `.chart-card` nur
`padding`/`border` setzt und seine Höhe sonst "auto", also vom Inhalt
abgeleitet ist), entsteht eine Rückkopplungsschleife: das Canvas wächst
→ der Elternknoten wächst mit, weil seine Höhe vom Canvas-Inhalt abhängt
→ der ResizeObserver sieht die neue (größere) Elternhöhe und vergrößert
das Canvas erneut → unendliches Wachstum nach unten, in der Praxis als
"ein Diagramm expandiert beim Laden der Seite endlos" sichtbar (gefunden
auf dem Dashboard beim `chart-verlauf`-Diagramm, das direktes Kind von
`.chart-card` war).

Ein direktes `canvas.style.height = "…px"` in JavaScript **behebt das
nicht** — im Gegenteil, es kollidiert mit Chart.js' eigener Verwaltung
von Canvas-Breite/-Höhe und kann die Schleife sogar erst auslösen. Die
Höhe gehört stattdessen auf einen eigenen Wrapper-`<div>` als direkten
Elternknoten des Canvas (`position: relative` plus feste oder dynamisch
per JS gesetzte Höhe), niemals auf das Canvas selbst. Siehe
`internal/web/static/app.css` (`.chart-canvas-wrap`, mit ausführlichem
Kommentar) und `internal/web/templates/pages/dashboard.html` — jedes der
vier Dashboard-Diagramme hat seinen eigenen `<div class="chart-canvas-
wrap" id="chart-<name>-wrap">` um das `<canvas>`. Bei fester Höhe (hier:
Nachrichtenvolumen, Disposition) reicht CSS; bei datenabhängiger Höhe
(hier: Top-Sendequellen, Heatmap — mehr Zeilen/Spalten brauchen mehr
Platz) setzt `internal/web/static/charts.js` die Höhe zur Laufzeit per
`document.getElementById("chart-<name>-wrap").style.height = …` auf den
Wrapper, nicht auf das Canvas. Bei jedem neuen Chart.js-Diagramm mit
`maintainAspectRatio: false` dieses Muster übernehmen, sonst tritt der
Bug erneut auf.

