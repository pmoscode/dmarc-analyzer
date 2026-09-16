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

## Bereits eingebunden (AP 0)

| Modul | Version | Verwendung |
| --- | --- | --- |
| `github.com/stretchr/testify` | `v1.12.1` | `require` in allen Tests ab AP 0 (siehe IMPLEMENTIERUNG.md Abschnitt 12.4) |

## Gepinnt für spätere Arbeitspakete

Bei Einführung in der jeweiligen Phase exakt diese Version anfragen
(`go get <modul>@<version>`), danach hier den Status auf „eingebunden"
aktualisieren. Falls zwischenzeitlich eine neuere Patch-/Minor-Version
erschienen ist: kurz prüfen (Changelog des Moduls), ob ein Update sinnvoll
ist, statt blind die hier notierte Version zu nehmen — dieser Stand ist ein
Ausgangspunkt, kein Dogma.

| Modul | Version (Stand 2026-09-16) | Geplant für | Hinweis |
| --- | --- | --- | --- |
| `fyne.io/fyne/v2` | `v2.8.1` | AP 5 | UI-Framework, Vorgabe aus `FEATURES.md` |
| `github.com/emersion/go-imap/v2` | `v2.0.0-beta.8` | AP 3 | Noch Beta — siehe Risiko in IMPLEMENTIERUNG.md Abschnitt 16, Version fest pinnen |
| `github.com/emersion/go-message` | `v0.18.2` | AP 1 (MIME-Anhänge) / AP 3 (IMAP-Nachrichten) | |
| `modernc.org/sqlite` | `v1.59.0` | AP 2 | CGO-frei |
| `github.com/zalando/go-keyring` | `v0.2.8` | AP 3 | OS-Schlüsselbund |
| `github.com/wcharczuk/go-chart/v2` | `v2.1.2` | AP 6 | Hinter `ChartRenderer`-Port gekapselt |

Standardbibliothek (`encoding/xml`, `compress/gzip`, `archive/zip`,
`database/sql`, `log/slog`, `fyne.io/fyne/v2/test`) braucht kein Pinning.
