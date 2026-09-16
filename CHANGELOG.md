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
