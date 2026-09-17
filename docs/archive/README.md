# Archiv: Planungsdokumente der Desktop-Ära

Die Dokumente in diesem Ordner stammen aus der Zeit, als `dmarc-analyzer`
als eigenständiges Desktop-Programm (zunächst mit Fyne, später mit
eingebetteter, aber weiterhin nur lokal auf `127.0.0.1` laufender
Web-Oberfläche) für macOS/Windows/Linux geplant und gebaut wurde:

- **FEATURES.md** — ursprüngliche Anforderungsliste, nannte Fyne als
  UI-Vorgabe.
- **IMPLEMENTIERUNG.md** — detaillierte technische Umsetzungsplanung
  (Architektur, Datenmodell, Sicherheitsmodell mit OS-Schlüsselbund) für
  die Arbeitspakete AP 0–7.
- **UMSETZUNGSPLAN.md** — Fortschritts-Tracking über die Arbeitspakete AP
  0–7 hinweg.
- **MIGRATIONSPLAN.md** — Umstieg von der Fyne-Desktop-Oberfläche auf die
  eingebettete Web-Oberfläche (Meilensteine M0–M6, siehe auch
  `docs/adr/0001-web-oberflaeche-statt-fyne.md`).

Mit `docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md` ist die
Anwendung auf ein reines Docker-Deployment mit OIDC/Authentik-Anmeldung
umgestellt worden — die in diesen Dokumenten beschriebenen Konzepte
(OS-Schlüsselbund, plattformkonforme Pfade, Ersteinrichtungs-Assistent,
Einzelinstanz-Erkennung, Release-Binärdateien für drei Plattformen) gibt es
im Code nicht mehr. Die Dokumente bleiben als historischer Kontext für die
ADRs erhalten, sind aber **keine Beschreibung des aktuellen Stands** mehr.

Aktuelle Dokumentation:

- `docs/architecture.md` — die weiterhin gültige Architektur (Clean
  Architecture, Domain/App/Infra-Schichtung).
- `docs/features/` — Feature-Dokumentation pro Domäne.
- `docs/adr/` — Architekturentscheidungen samt Begründung.
- `README.md` (Repository-Wurzel) — Kurzüberblick und Einstieg.
