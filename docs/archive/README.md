# Archive: planning documents from the desktop era

The documents in this folder date from when `dmarc-analyzer` was planned
and built as a standalone desktop program (initially with Fyne, later
with an embedded but still only-locally-on-`127.0.0.1` web UI) for
macOS/Windows/Linux:

- **FEATURES.md** — original requirements list, named Fyne as the UI
  requirement.
- **IMPLEMENTIERUNG.md** — detailed technical implementation planning
  (architecture, data model, security model with OS keychain) for work
  packages WP 0–7.
- **UMSETZUNGSPLAN.md** — progress tracking across work packages WP 0–7.
- **MIGRATIONSPLAN.md** — move from the Fyne desktop UI to the embedded
  web UI (milestones M0–M6, see also
  `docs/adr/0001-web-oberflaeche-statt-fyne.md`).

With `docs/adr/0003-docker-nativ-oidc-statt-desktop-keychain.md`, the
application was moved to a pure Docker deployment with OIDC/Authentik
login — the concepts described in these documents (OS keychain,
platform-appropriate paths, first-run wizard, single-instance detection,
release binaries for three platforms) no longer exist in the code. These
documents remain as historical context for the ADRs, but are **not** a
description of the current state anymore.

Current documentation:

- `docs/architecture.md` — the still-valid architecture (Clean
  Architecture, domain/app/infra layering).
- `docs/features/` — per-domain feature documentation.
- `docs/adr/` — architecture decisions with rationale.
- `README.md` (repository root) — short overview and entry point.
