# Abhängigkeiten

> Ergänzt `IMPLEMENTIERUNG.md` Abschnitt 3. Dort steht die *Begründung* je
> Bibliothek, hier die *konkret gepinnte Version* — recherchiert am
> 2026-09-16 gegen `proxy.golang.org`.

## Grundsatz

Eine Abhängigkeit wird erst per `go get <modul>@<version>` in `go.mod`
aufgenommen, wenn tatsächlich Code sie importiert. Ein `go.mod`, das
Abhängigkeiten enthält, die nirgends importiert werden, lügt über den
Ist-Zustand und wird beim nächsten `task tidy` ohnehin wieder entfernt.
Die Tabelle unten ist deshalb der **Bestellzettel für die jeweilige Phase**,
nicht der aktuelle `go.mod`-Inhalt.

## Bereits eingebunden

| Modul | Version | Verwendung |
| --- | --- | --- |
| `github.com/stretchr/testify` | `v1.12.1` | `require` in allen Tests ab AP 0 (siehe IMPLEMENTIERUNG.md Abschnitt 12.4) |
| `modernc.org/sqlite` | `v1.59.0` | Persistenz (AP 2), CGO-frei — genau die hier vorab recherchierte Version |
| `github.com/emersion/go-imap/v2` | `v2.0.0-beta.8` | IMAP-Adapter (AP 3, `internal/infra/imap`) — genau die vorab recherchierte Version |
| `github.com/zalando/go-keyring` | `v0.2.8` | OS-Schlüsselbund-Adapter (AP 3, `internal/infra/keyring.OSStore`) — genau die vorab recherchierte Version |
| `golang.org/x/crypto` | `v0.57.0` | `scrypt` für den Linux-Datei-Fallback (AP 3, `internal/infra/keyring.FileStore`). **Nicht vorab recherchiert** — in `IMPLEMENTIERUNG.md` Abschnitt 3 nicht gelistet, weil AES-256-GCM selbst aus `crypto/aes`/`crypto/cipher` (Standardbibliothek) kommt; nur die Schlüsselableitung per scrypt braucht `x/crypto`, das keine eigene RFC-7489-artige Reifediskussion nötig hatte — offizielles, vom Go-Team gepflegtes Erweiterungsmodul, hier ohne Weiteres wie Standardbibliothek behandelt. |
| `github.com/emersion/go-message` | `v0.18.2` | MIME-Zerlegung (AP 4, `internal/infra/mailmime`) zu `sync.RawAttachment` — genau die vorab recherchierte Version, jetzt direkt importiert (vorher nur transitiv über `go-imap/v2/imapclient`). |
| `github.com/google/uuid` | `v1.6.0` | Konto-IDs beim Anlegen (AP 4, `cmd/dmarc-analyzer` CLI `account add`). War schon vorher transitiv vorhanden (über `modernc.org/sqlite`), jetzt direkt importiert. Nicht vorab recherchiert — offizielles, weit verbreitetes Google-Modul ohne eigene Versionsdiskussion nötig, wie `x/crypto` oben. |
| `fyne.io/fyne/v2` | `v2.8.1` | UI-Framework (AP 5), jetzt zusätzlich `fyne.io/fyne/v2/app` (AP 6, `cmd/dmarc-analyzer/cmd_gui.go`) für den echten (nicht Test-)Treiber — genau die vorab recherchierte Version, jetzt direkt importiert. |
| `github.com/wcharczuk/go-chart/v2` | `v2.1.2` | `ChartRenderer`-Adapter (AP 6, `internal/infra/charts`) — genau die vorab recherchierte Version, jetzt direkt importiert. |

### Frontend (Web-Migration, `internal/web/static/vendor/`) — kein Go-Modul, minifizierte Dateien im Repository

`MIGRATIONSPLAN.md` E-7: statt CDN werden htmx, Chart.js und dessen Plugins
als minifizierte UMD-Bündel direkt im Repository abgelegt (offline-fähig,
passt zur Content-Security-Policy `default-src 'self'`). Lizenzen liegen
jeweils als `LICENSE.<name>.txt` daneben.

| Datei | Version | Lizenz | Verwendung |
| --- | --- | --- | --- |
| `chart.umd.min.js` | `4.5.1` | MIT | Diagramme im Browser (`internal/web/static/charts.js`, MIGRATIONSPLAN.md Abschnitt 6a) |
| `chartjs-chart-matrix.min.js` | `3.1.0` | MIT | Heatmap-Diagrammtyp (`matrix`) — Kompatibilität zu Chart.js 4.x geprüft (Peer-Dependency `^4.0.0`) |
| `chartjs-plugin-zoom.min.js` | `2.2.0` | MIT | Zoom/Verschieben in der Zeitreihe — Kompatibilität zu Chart.js 4.x geprüft (Peer-Dependency `^4.0.0`); braucht kein `hammerjs` für Maus-/Touch-Pad-Zoom, nur für Pinch-Gesten auf Touch-Geräten (hier nicht eingebunden, Zoom per Mausrad/Trackpad reicht) |
| `htmx.min.js` | `2.0.10` | BSD-0-Clause (Zero-Clause BSD) | Serverseitig gerenderte Teilaktualisierungen (Paginierung, Dialoge — ab Meilenstein M2/M3) |

Bei einem Versions-Update: alle drei Chart.js-Dateien (Kern + beide
Plugins) gemeinsam aktualisieren und gegeneinander testen (Plugins folgen
nicht zwingend demselben Versionsschema wie Chart.js selbst) — siehe
MIGRATIONSPLAN.md Risiko-Tabelle Abschnitt 13.

## Gepinnt für spätere Arbeitspakete

Bei Einführung in der jeweiligen Phase exakt diese Version anfragen
(`go get <modul>@<version>`), danach hier den Status auf „eingebunden"
aktualisieren. Falls zwischenzeitlich eine neuere Patch-/Minor-Version
erschienen ist: kurz prüfen (Changelog des Moduls), ob ein Update sinnvoll
ist, statt blind die hier notierte Version zu nehmen — dieser Stand ist ein
Ausgangspunkt, kein Dogma.

Aktuell nichts offen — alle in `IMPLEMENTIERUNG.md` Abschnitt 3 vorab
recherchierten Abhängigkeiten sind eingebunden. Für AP 7 (Packaging) wird
voraussichtlich nichts Neues gebraucht — `task release`/`fyne package`
kommen mit dem `fyne`-CLI-Werkzeug, keiner zusätzlichen `go.mod`-Abhängigkeit.

Standardbibliothek (`encoding/xml`, `compress/gzip`, `archive/zip`,
`database/sql`, `log/slog`, `crypto/aes`, `crypto/cipher`,
`fyne.io/fyne/v2/test`) braucht kein Pinning.
