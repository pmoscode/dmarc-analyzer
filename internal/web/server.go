// Package web implements the embedded web UI. Only calls use cases from
// internal/app, never a concrete infra adapter directly (AGENTS.md).
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

// Options controls the address, template source, and the Authentik
// integration.
type Options struct {
	// Addr is the address the HTTP server binds to (e.g. ":8080") — comes
	// from DMARC_LISTEN_ADDR, see internal/infra/envconfig. Unlike before
	// the move to Docker, this deliberately has NO restriction to loopback
	// addresses anymore: the container must be reachable from outside,
	// access control runs via OIDC (see auth.go/oidc.go) instead of "only
	// reachable from the same machine".
	Addr string
	// Dev reads templates/static assets from disk instead of embedded —
	// for development without a rebuild on every change (the working
	// directory must be the repository root), from DMARC_DEV_MODE.
	Dev bool
	// Logger — nil uses slog.Default().
	Logger *slog.Logger
	// OIDC holds the parameters for the Authentik login.
	OIDC OIDCConfig
	// Build holds the version and git commit of the running build, shown
	// in the header below the wordmark on every page (layout.html).
	Build BuildInfo
}

// Server is the embedded web UI.
type Server struct {
	deps    Dependencies
	logger  *slog.Logger
	auth    *auth
	oidc    *oidcAuthenticator
	views   *views
	devMode bool

	// allowedHost is the public hostname (from OIDC.RedirectURL) that
	// requireHost checks incoming requests' Host header against (see
	// middleware.go) — replaces the former allowlist computed from the
	// bound loopback address.
	allowedHost string
	// oidcIssuerOrigin is the origin (scheme://host) of the OIDC issuer —
	// added to the CSP form-action directive so the RP-initiated logout
	// redirect to Authentik's end_session_endpoint isn't blocked by the
	// browser (see middleware.go:securityHeaders).
	oidcIssuerOrigin string

	staticFS fs.FS

	httpServer *http.Server
	listener   net.Listener
}

// New builds the server (loads templates, OIDC discovery against
// Authentik), but doesn't bind a port yet — that's Start()'s job.
// Separated so construction errors (e.g. Authentik unreachable, broken
// template) can be reported without a network side effect.
func New(ctx context.Context, deps Dependencies, opts Options) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	redirectURL, err := url.Parse(opts.OIDC.RedirectURL)
	if err != nil || redirectURL.Host == "" {
		return nil, fmt.Errorf("DMARC_OIDC_REDIRECT_URL is not a complete URL: %q", opts.OIDC.RedirectURL)
	}

	issuerURL, err := url.Parse(opts.OIDC.IssuerURL)
	if err != nil || issuerURL.Host == "" {
		return nil, fmt.Errorf("DMARC_OIDC_ISSUER_URL is not a complete URL: %q", opts.OIDC.IssuerURL)
	}

	authenticator, err := newOIDCAuthenticator(ctx, opts.OIDC)
	if err != nil {
		return nil, err
	}

	a := newAuth()

	v, err := newViews(opts.Dev, opts.Build, a.csrfTokenForRequest)
	if err != nil {
		return nil, fmt.Errorf("could not load templates: %w", err)
	}

	sfs, err := staticFS(opts.Dev)
	if err != nil {
		return nil, err
	}

	s := &Server{
		deps:             deps,
		logger:           logger,
		auth:             a,
		oidc:             authenticator,
		views:            v,
		devMode:          opts.Dev,
		allowedHost:      redirectURL.Host,
		oidcIssuerOrigin: issuerURL.Scheme + "://" + issuerURL.Host,
		staticFS:         sfs,
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

// bind binds the server to addr.
func (s *Server) bind(addr string) error {
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("could not bind address %q: %w", addr, err)
	}
	s.listener = ln
	return nil
}

// Start starts the server in the background. Lifecycle managed via
// SIGINT/SIGTERM (see cmd/dmarc-analyzer/cmd_web.go) — "docker stop"
// sends SIGTERM.
func (s *Server) Start(context.Context) error {
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("server stopped", "error", err)
		}
	}()
	return nil
}

// Shutdown shuts the server down cleanly (finishes open requests).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the actually bound address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}
