# ADR 0001: Eingebettete Web-Oberfläche statt Fyne

- Status: angenommen
- Datum: 2026-09-17 (Migrationsentscheidung), umgesetzt in `MIGRATIONSPLAN.md`
  M0–M5 (Fyne endgültig entfernt in M5)

## Kontext

`dmarc-analyzer` war ursprünglich ein Desktop-Programm mit
[Fyne](https://fyne.io/) als UI-Toolkit (`FEATURES.md` nannte Fyne
ausdrücklich als Vorgabe, `IMPLEMENTIERUNG.md` Abschnitt 1.2 behandelte das
als verbindlich). In der Praxis zeigten sich mehrere Probleme:

- Die Oberfläche wirkte trotz mehrfacher Überarbeitung (eigenes Theme,
  Farbpalette, Kartenlayout) weiterhin schwerfällig/"klobig".
- Fyne ist die **einzige CGO-Abhängigkeit** des gesamten Moduls — geprüft am
  2026-09-17: ohne `internal/ui` bauen alle Pakete bereits mit
  `CGO_ENABLED=0` für `linux/amd64`, `windows/amd64` und `darwin/arm64`.
  Cross-Compile-Releases ohne native Toolchains pro Zielplattform waren mit
  Fyne nicht ohne Weiteres möglich.
- Fyne v2.8 bietet keine echten Tooltips (Workaround: ein eigener
  "?"-Knopf neben jedem Diagramm/jeder Kennzahl, siehe die inzwischen
  gelöschten `internal/ui/glossary`-Verweise).
- Diagramme entstanden serverseitig als statische PNGs (`go-chart`,
  `internal/infra/charts`) — kein Tooltip, kein Zoom, kein Klick-Drilldown,
  fester weißer Hintergrund (`flattenOnWhite`) auch im dunklen Systemmodus.
- `internal/ui` sammelte im Lauf der Zeit mehrere eigene Fyne-Fallstricke an
  (dokumentiert und dann mit dem Paket wieder entfernt aus `AGENTS.md`:
  `TestMain`/`test.NewApp()`, `runBackground`-Injektion für Data-Race-freie
  Tests, `container.NewBorder`-Indexreihenfolge,
  `widget.Select.SetSelected()`-Callback-Timing,
  `uitest.FindEntries`-Typassertion, `dialog.NewCustomWithoutButtons`).
  Jede dieser Fallen kostete Zeit und war reines Fyne-API-Wissen ohne
  fachlichen Mehrwert.

## Entscheidung

Die gesamte Präsentationsschicht wird durch eine **im Programm eingebettete
Web-Oberfläche** ersetzt: Beim Start läuft ein lokaler HTTP-Server
(`internal/web`, `net/http` + `html/template` aus der Standardbibliothek,
kein Bundler, kein Node-Werkzeug), der Standardbrowser öffnet automatisch
die Hauptseite. Erwogene Alternative war eine SPA (Svelte/Vue/React) mit
eigenem Vite-Build — verworfen, weil sie ein zweites Werkzeug-Ökosystem und
eigene Tests bräuchte, während serverseitiges Rendern für Tabellen und
Formulare gut ausreicht (`MIGRATIONSPLAN.md` Entscheidung E-1).

Die Migration lief in Meilensteinen (`MIGRATIONSPLAN.md` M0–M5): zuerst
`internal/web` parallel zu `internal/ui` aufbauen (Entscheidung E-5), bei
Funktionsgleichheit umschalten (M3: `dmarc-analyzer` ohne Argumente startet
die Web-Oberfläche statt Fyne), dann in M5 `internal/ui`,
`cmd/dmarc-analyzer/cmd_gui.go` und `fyne.io/fyne/v2` (samt aller
transitiven Fyne-Abhängigkeiten) vollständig entfernen.

Der Lebenszyklus folgt bewusst dem Server-Modell, nicht dem
Desktop-App-Modell: nur Strg+C/SIGTERM beenden den Prozess (kein
"Beenden"-Knopf, kein Auto-Ende nach Leerlauf, kein Tray-Icon) — siehe
`MIGRATIONSPLAN.md` Entscheidung E-3 und den zugehörigen Risiko-Eintrag in
Abschnitt 13 ("Server läuft unbemerkt weiter, wenn nur der Browser-Tab
geschlossen wird" — bewusst in Kauf genommen).

## Konsequenzen

**Vorteile:**

- Keine CGO-Abhängigkeit mehr im gesamten Modul; `task release` baut alle
  Zielplattformen (macOS arm64/amd64, Windows amd64, Linux amd64/arm64) per
  reinem `CGO_ENABLED=0 go build ...` auf einem einzigen Rechner, ohne
  plattformspezifische Toolchains.
- Interaktive Diagramme mit Tooltips, umschaltbarer Legende, Zoom und
  Klick-Drilldown (siehe ADR 0002) statt statischer PNGs.
- Diagramme passen sich automatisch an hell/dunkel an (Chart.js liest
  CSS-Variablen).
- Filter stehen in der URL — Lesezeichen und Zurück-Knopf funktionieren.
- Alle Fyne-spezifischen Testfallen (siehe oben) sind mit dem Paket
  verschwunden; die Web-Schicht hat eigene, dafür allgemeinere Fallstricke
  (dokumentiert in `AGENTS.md`, z. B. CSP/CSRF/Chart.js-Resize).

**Nachteile / bewusst in Kauf genommen:**

- Die Oberfläche wirkt nicht mehr "nativ" (kein systemeigenes
  Fenster-Look-and-Feel) — bewusst akzeptiert, siehe
  `MIGRATIONSPLAN.md` Risiko-Tabelle.
- Ein Server auf `127.0.0.1` ist nicht automatisch privat (andere
  Benutzer desselben Rechners, DNS-Rebinding, CSRF von anderen offenen
  Webseiten) — braucht ein eigenes Sicherheitsmodell (Einmal-Anmeldelink,
  Sitzungs-Cookie, `Host`-Prüfung, CSRF-Token, CSP), umgesetzt in M1 und in
  `MIGRATIONSPLAN.md` Abschnitt 5 ausführlich begründet. Ein reines
  Desktop-Programm mit eigenem Fenster hätte dieses Problem nicht gehabt.
- Ohne eigenes Display in der Entwicklungsumgebung, in der M4/M5 entstanden,
  ließen sich manche Verhaltensweisen (Browser-Smoke-Test, macOS-Dock-
  Verhalten des `.app`-Bündels) nicht abschließend visuell verifizieren —
  dokumentiert an den jeweiligen Stellen (`MIGRATIONSPLAN.md`,
  `packaging/darwin/Info.plist.tmpl`).

## Alternativen

- **Fyne beibehalten, nur Diagramme/Tooltips verbessern:** hätte das
  CGO-Problem und die "klobige" Anmutung nicht gelöst.
- **SPA mit eigenem Frontend-Build (Svelte/Vue/React):** verworfen, siehe
  Entscheidung E-1 oben — unverhältnismäßiger Mehraufwand für ein
  Ein-Personen-/Kleinteam-Werkzeug mit überwiegend Tabellen/Formularen.
- **Electron/Tauri (Web-Technik im eigenen Fenster):** nicht ernsthaft
  erwogen — bringt ein eigenes Laufzeit-/Bündelungsproblem zurück (bei
  Electron zusätzlich Chromium im Lieferumfang), ohne die Vorteile "läuft
  bereits im Browser des Nutzers, keine zusätzliche Runtime" zu behalten.
