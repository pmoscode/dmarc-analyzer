package web

import "net/http"

// routes builds the full handler tree. Middleware order (outer to
// inner): panic recovery, security headers, then either /gesund (see
// below) or host check + routing for everything else.
//
// /gesund is deliberately BEFORE requireHost, not just before
// requireSession: Docker's HEALTHCHECK (see cmd_healthcheck.go) connects
// within the container via "127.0.0.1:<port>", so the Host header never
// carries the public hostname from DMARC_OIDC_REDIRECT_URL — with
// requireHost in front of it, the container would be permanently
// "unhealthy" (actually reproduced via "docker run"). A healthcheck is
// also not security-critical (only reports "is the process running", no
// data), so requireHost's DNS-rebinding protection isn't needed here.
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
	protected.HandleFunc("GET /fehlschlaege", s.handleFailedRecords)
	protected.HandleFunc("GET /fehlschlaege/seite", s.handleFailedRecordsPage)
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
	protected.HandleFunc("GET /export/fehlschlaege.csv", s.handleExportFailedRecordsCSV)
	protected.HandleFunc("POST /abmelden", s.handleLogout)

	hostChecked.Handle("/", requireSession(s.auth, requireCSRF(s.auth, protected)))

	// /static/ is deliberately outside requireSession: CSS/JS aren't
	// sensitive, and the login page itself needs them before a session
	// exists. Still behind requireHost, unlike /gesund above — unlike the
	// healthcheck, a request here always comes through the browser with a
	// real Host header.
	hostChecked.Handle("/static/", http.StripPrefix("/static/", s.staticHandler()))

	mux.Handle("/", requireHost(s.allowedHost, hostChecked))

	return recoverPanic(s.logger, s.securityHeaders(mux))
}
