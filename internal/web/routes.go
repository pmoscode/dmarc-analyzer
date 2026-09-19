package web

import "net/http"

// routes baut den vollständigen Handler-Baum. Middleware-Reihenfolge
// (außen nach innen): Panic-Sicherung, Sicherheits-Header, dann erst
// /gesund (siehe unten) bzw. Host-Prüfung + Routing für alles andere.
//
// /gesund liegt bewusst VOR requireHost, nicht nur vor requireSession:
// Dockers HEALTHCHECK (siehe cmd_healthcheck.go) verbindet sich
// containerintern über "127.0.0.1:<port>", der Host-Header trägt also nie
// den öffentlichen Hostnamen aus DMARC_OIDC_REDIRECT_URL — mit
// requireHost davor wäre der Container dauerhaft "unhealthy" (per
// "docker run" tatsächlich reproduziert). Ein Healthcheck ist außerdem
// nicht sicherheitskritisch (liefert nur "läuft der Prozess", keine
// Daten), die DNS-Rebinding-Schutzwirkung von requireHost wird hier nicht
// gebraucht.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /gesund", s.handleHealthz)

	hostChecked := http.NewServeMux()
	hostChecked.HandleFunc("GET /anmelden", s.handleLoginStart)
	hostChecked.HandleFunc("GET /anmelden/callback", s.handleLoginCallback)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /{$}", s.handleDashboard)
	protected.HandleFunc("GET /api/diagramme/verlauf", s.handleChartDailyVolume)
	protected.HandleFunc("GET /api/diagramme/heatmap", s.handleChartHeatmap)
	protected.HandleFunc("GET /api/diagramme/quellen", s.handleChartTopSources)
	protected.HandleFunc("GET /api/diagramme/disposition", s.handleChartDisposition)
	protected.HandleFunc("GET /berichte", s.handleReports)
	protected.HandleFunc("GET /berichte/seite", s.handleReportsPage)
	protected.HandleFunc("GET /berichte/{id}", s.handleReportDetail)
	protected.HandleFunc("GET /quellen", s.handleSources)
	protected.HandleFunc("GET /quellen/seite", s.handleSourcesPage)
	protected.HandleFunc("GET /domains", s.handleDomains)
	protected.HandleFunc("GET /domains/seite", s.handleDomainsPage)
	protected.HandleFunc("GET /einstellungen", s.handleSettings)
	protected.HandleFunc("POST /konten/{id}/test", s.handleAccountTest)
	protected.HandleFunc("POST /abgleich", s.handleSyncStart)
	protected.HandleFunc("POST /abgleich/abbrechen", s.handleSyncCancel)
	protected.HandleFunc("GET /ereignisse", s.handleEvents)
	protected.HandleFunc("GET /import", s.handleImportForm)
	protected.HandleFunc("POST /import", s.handleImportSubmit)
	protected.HandleFunc("GET /export/berichte.csv", s.handleExportReportsCSV)
	protected.HandleFunc("GET /export/quellen.csv", s.handleExportSourcesCSV)
	protected.HandleFunc("GET /export/domains.csv", s.handleExportDomainsCSV)
	protected.HandleFunc("POST /abmelden", s.handleLogout)

	hostChecked.Handle("/", requireSession(s.auth, requireCSRF(s.auth, protected)))

	// /static/ bewusst außerhalb von requireSession: CSS/JS sind nicht
	// schützenswert, und die Anmeldeseite selbst braucht sie, bevor eine
	// Sitzung existiert. Trotzdem hinter requireHost, anders als /gesund
	// oben — anders als der Healthcheck kommt eine Anfrage hierfür immer
	// über den Browser mit echtem Host-Header.
	hostChecked.Handle("/static/", http.StripPrefix("/static/", s.staticHandler()))

	mux.Handle("/", requireHost(s.allowedHost, hostChecked))

	return recoverPanic(s.logger, s.securityHeaders(mux))
}
