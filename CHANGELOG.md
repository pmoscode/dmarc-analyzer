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
