package web

import "net/http"

// routes baut den vollständigen Handler-Baum. Middleware-Reihenfolge
// (außen nach innen): Panic-Sicherung, Sicherheits-Header, Host-Prüfung,
// dann erst das eigentliche Routing — /anmelden und /intern/code
// ausgenommen von Sitzungsprüfung und CSRF (siehe dort, eigenes
// Geheimnis-basiertes Verfahren statt Cookie-Sitzung), alles andere
// unter "/" durchläuft zusätzlich requireSession und requireCSRF
// (MIGRATIONSPLAN.md Abschnitt 5 und 7).
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /anmelden", s.handleLogin)
	// POST /intern/code ist bewusst außerhalb von requireSession/protected
	// verdrahtet (wie /anmelden) — es authentifiziert sich über das
	// Instanz-Geheimnis, nicht über eine Sitzung (siehe instance.go).
	mux.HandleFunc("POST /intern/code", s.handleInternalCode)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /{$}", s.handleDashboard)
	protected.HandleFunc("GET /api/diagramme/verlauf", s.handleChartDailyVolume)
	protected.HandleFunc("GET /api/diagramme/heatmap", s.handleChartHeatmap)
	protected.HandleFunc("GET /api/diagramme/quellen", s.handleChartTopSources)
	protected.HandleFunc("GET /api/diagramme/disposition", s.handleChartDisposition)
	// Platzhalter-Seiten (Meilenstein M1: "Navigation zwischen leeren
	// Seiten") — Inhalt folgt in M2 (Berichte, Sendequellen, Glossar)
	// bzw. M3 (Einstellungen).
	protected.HandleFunc("GET /berichte", s.handleReports)
	protected.HandleFunc("GET /berichte/seite", s.handleReportsPage)
	protected.HandleFunc("GET /berichte/{id}", s.handleReportDetail)
	protected.HandleFunc("GET /quellen", s.handleSources)
	protected.HandleFunc("GET /quellen/seite", s.handleSourcesPage)
	protected.HandleFunc("GET /glossar", s.handleGlossary)
	protected.HandleFunc("GET /einstellungen", s.handlePlaceholder("Einstellungen", "Die Kontenverwaltung kommt in Kürze."))

	mux.Handle("/", requireSession(s.auth, requireCSRF(s.auth, protected)))

	// /static/ bewusst außerhalb von requireSession: CSS/JS sind nicht
	// schützenswert, und die Anmeldeseite selbst braucht sie, bevor eine
	// Sitzung existiert.
	mux.Handle("/static/", http.StripPrefix("/static/", s.staticHandler()))

	return recoverPanic(s.logger, securityHeaders(requireHost(s.allowedHosts, mux)))
}
