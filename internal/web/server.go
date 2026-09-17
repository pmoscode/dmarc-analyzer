// Package web implementiert die eingebettete Web-Oberfläche. Ruft
// ausschließlich Use Cases aus internal/app auf, nie direkt einen
// Infra-Adapter (AGENTS.md).
package web

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Options steuert Adresse, Vorlagen-Quelle und die Authentik-Anbindung.
type Options struct {
	// Addr ist die Adresse, auf die der HTTP-Server bindet (z. B.
	// ":8080") — kommt aus DMARC_LISTEN_ADDR, siehe
	// internal/infra/envconfig. Anders als vor dem Umstieg auf Docker ist
	// hier bewusst KEINE Beschränkung auf Loopback-Adressen mehr
	// eingebaut: der Container muss von außerhalb erreichbar sein, der
	// Zugriffsschutz läuft über OIDC (siehe auth.go/oidc.go) statt über
	// "nur vom selben Rechner erreichbar".
	Addr string
	// Dev liest Vorlagen/Statik von der Festplatte statt eingebettet —
	// für Entwicklung ohne Neubau bei jeder Änderung (Arbeitsverzeichnis
	// muss die Repository-Wurzel sein), aus DMARC_DEV_MODE.
	Dev bool
	// Logger — nil verwendet slog.Default().
	Logger *slog.Logger
	// OIDC sind die Parameter für die Authentik-Anmeldung.
	OIDC OIDCConfig
}

// Server ist die eingebettete Web-Oberfläche.
type Server struct {
	deps    Dependencies
	logger  *slog.Logger
	auth    *auth
	oidc    *oidcAuthenticator
	views   *views
	devMode bool

	// allowedHost ist der öffentliche Hostname (aus OIDC.RedirectURL), den
	// requireHost gegen den Host-Header eingehender Anfragen prüft (siehe
	// middleware.go) — ersetzt die frühere, aus der gebundenen
	// Loopback-Adresse berechnete Zulassungsliste.
	allowedHost string

	staticFS fs.FS

	httpServer *http.Server
	listener   net.Listener
}

// New baut den Server auf (Vorlagen laden, OIDC-Discovery gegen
// Authentik), bindet aber noch keinen Port — das übernimmt Start().
// Getrennt, damit Konstruktionsfehler (z. B. Authentik nicht erreichbar,
// kaputte Vorlage) sich ohne Netzwerk-Seiteneffekt melden.
func New(ctx context.Context, deps Dependencies, opts Options) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	redirectURL, err := url.Parse(opts.OIDC.RedirectURL)
	if err != nil || redirectURL.Host == "" {
		return nil, fmt.Errorf("DMARC_OIDC_REDIRECT_URL ist keine vollständige URL: %q", opts.OIDC.RedirectURL)
	}

	authenticator, err := newOIDCAuthenticator(ctx, opts.OIDC)
	if err != nil {
		return nil, err
	}

	a := newAuth()

	v, err := newViews(opts.Dev, a.csrfTokenForRequest)
	if err != nil {
		return nil, fmt.Errorf("vorlagen konnten nicht geladen werden: %w", err)
	}

	sfs, err := staticFS(opts.Dev)
	if err != nil {
		return nil, err
	}

	s := &Server{
		deps:        deps,
		logger:      logger,
		auth:        a,
		oidc:        authenticator,
		views:       v,
		devMode:     opts.Dev,
		allowedHost: redirectURL.Host,
		staticFS:    sfs,
	}
	s.httpServer = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := s.bind(opts.Addr); err != nil {
		return nil, err
	}

	return s, nil
}

// bind bindet den Server an addr.
func (s *Server) bind(addr string) error {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("adresse %q konnte nicht gebunden werden: %w", addr, err)
	}
	s.listener = ln
	return nil
}

// Start startet den Server im Hintergrund. Lebenszyklus über
// SIGINT/SIGTERM (siehe cmd/dmarc-analyzer/cmd_web.go) — "docker stop"
// sendet SIGTERM.
func (s *Server) Start(context.Context) error {
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server beendet", "error", err)
		}
	}()
	return nil
}

// Shutdown fährt den Server sauber herunter (offene Anfragen fertig).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr liefert die tatsächlich gebundene Adresse.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}
