# Abhängigkeiten

> Ergänzt `docs/architecture.md`. Dort steht die *Begründung* je Bibliothek
> in groben Zügen, hier die *konkret gepinnte Version*.

## Grundsatz

Eine Abhängigkeit wird erst per `go get <modul>@<version>` in `go.mod`
aufgenommen, wenn tatsächlich Code sie importiert. Ein `go.mod`, das
Abhängigkeiten enthält, die nirgends importiert werden, lügt über den
Ist-Zustand und wird beim nächsten `task tidy` ohnehin wieder entfernt.

## Eingebunden

| Modul | Version | Verwendung |
| --- | --- | --- |
| `github.com/stretchr/testify` | `v1.12.1` | `require` in allen Tests |
| `modernc.org/sqlite` | `v1.59.0` | Persistenz (`internal/infra/sqlite`), CGO-frei — Voraussetzung für `CGO_ENABLED=0` im Docker-Build |
| `github.com/emersion/go-imap/v2` | `v2.0.0-beta.8` | IMAP-Adapter (`internal/infra/imap`) |
| `github.com/emersion/go-message` | `v0.18.2` | MIME-Zerlegung (`internal/infra/mailmime`) zu `sync.RawAttachment` |
| `github.com/coreos/go-oidc/v3` | `v3.21.0` | OIDC-Client (Discovery, ID-Token-Validierung) gegen Authentik, siehe `docs/features/auth.md` |
| `golang.org/x/oauth2` | `v0.37.0` | Authorization-Code-Flow + PKCE für die OIDC-Anmeldung |
| `github.com/go-jose/go-jose/v4` | `v4.1.5` | Transitiv über `go-oidc` (JWT/JWS-Signaturprüfung), direkt importiert im Test-Doppel `internal/web/fakeoidc_test.go` (lokaler Fake-Identity-Provider) |

### Frontend (`internal/web/static/vendor/`) — kein Go-Modul, minifizierte Dateien im Repository

Statt CDN werden htmx, Chart.js und dessen Plugins als minifizierte
UMD-Bündel direkt im Repository abgelegt (offline-fähig, passt zur
Content-Security-Policy `default-src 'self'`). Lizenzen liegen jeweils als
`LICENSE.<name>.txt` daneben.

| Datei | Version | Lizenz | Verwendung |
| --- | --- | --- | --- |
| `chart.umd.min.js` | `4.5.1` | MIT | Diagramme im Browser (`internal/web/static/charts.js`) |
| `chartjs-chart-matrix.min.js` | `3.1.0` | MIT | Heatmap-Diagrammtyp (`matrix`) |
| `chartjs-plugin-zoom.min.js` | `2.2.0` | MIT | Zoom/Verschieben in der Zeitreihe |
| `htmx.min.js` | `2.0.10` | BSD-0-Clause | Serverseitig gerenderte Teilaktualisierungen (Paginierung, Dialoge) |

Bei einem Versions-Update: alle drei Chart.js-Dateien (Kern + beide
Plugins) gemeinsam aktualisieren und gegeneinander testen (Plugins folgen
nicht zwingend demselben Versionsschema wie Chart.js selbst).

Standardbibliothek (`encoding/xml`, `compress/gzip`, `archive/zip`,
`database/sql`, `log/slog`, `net/http`) braucht kein Pinning.

## Entfernt

- `fyne.io/fyne/v2` und `github.com/wcharczuk/go-chart/v2` — mit dem
  Umstieg von der Fyne-Desktop-Oberfläche auf die eingebettete Web-
  Oberfläche entfernt. Begründung: `docs/adr/0001-web-oberflaeche-statt-fyne.md`,
  `docs/adr/0002-chartjs-statt-chartrenderer-port.md`.
- `github.com/zalando/go-keyring`, `golang.org/x/crypto` (scrypt),
  `github.com/danieljoos/wincred`, `github.com/godbus/dbus/v5` — der
  OS-Schlüsselbund-Adapter (`internal/infra/keyring`) ist mit dem Umstieg
  auf reine ENV-Konfiguration entfallen; IMAP-Zugangsdaten kommen jetzt
  aus `DMARC_IMAP_PASSWORD` statt aus einem persistierten Schlüsselbund.
  Begründung: `docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`.
- `github.com/google/uuid` — wurde nur für frei vergebene Konto-IDs beim
  interaktiven Anlegen eines Kontos gebraucht; es gibt seit dem
  ENV-Konfigurationsmodell nur noch ein Konto mit fester ID pro Container.
