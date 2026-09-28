package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the parameters for the Authentik login (see
// internal/infra/envconfig.OIDC) — its own type here so this package
// doesn't need to depend on internal/infra/envconfig (AGENTS.md: web
// only knows its own types/ports, no concrete infra packages).
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AdminGroup is the Authentik group name that the ID token's "groups"
	// claim must contain.
	AdminGroup string
	// InsecureSkipVerify disables TLS certificate verification for all
	// HTTP calls against the issuer (discovery, JWKS, token exchange).
	// Intended only for development environments with a self-signed
	// certificate — never set this in production.
	InsecureSkipVerify bool
}

// oidcAuthenticator encapsulates discovery against Authentik as well as
// exchanging/validating tokens (authorization code flow + PKCE, see
// handlers_login.go).
type oidcAuthenticator struct {
	provider   *oidc.Provider
	verifier   *oidc.IDTokenVerifier
	oauth2Cfg  oauth2.Config
	adminGroup string
	// httpClient, if set, is passed into every discovery/JWKS/token call
	// via oidc.ClientContext (see insecureContext) — nil means "default
	// http.Client with the system trust store".
	httpClient *http.Client
}

// insecureContext attaches o.httpClient (if set) to ctx via
// oidc.ClientContext — both coreos/go-oidc and golang.org/x/oauth2 read
// the HTTP client from the same context key (oidc.ClientContext is a
// direct wrapper around oauth2.NewClient) and use it for JWKS fetches and
// token exchange respectively.
func (o *oidcAuthenticator) insecureContext(ctx context.Context) context.Context {
	if o.httpClient == nil {
		return ctx
	}
	return oidc.ClientContext(ctx, o.httpClient)
}

// newOIDCAuthenticator loads the discovery document from cfg.IssuerURL —
// if that fails (Authentik unreachable, wrong issuer URL), the caller
// reports it as a clear startup error instead of a broken login at
// runtime.
func newOIDCAuthenticator(ctx context.Context, cfg OIDCConfig) (*oidcAuthenticator, error) {
	var httpClient *http.Client
	if cfg.InsecureSkipVerify {
		httpClient = &http.Client{
			Transport: &http.Transport{
				//nolint:gosec // G402: deliberate opt-in via
				// DMARC_OIDC_INSECURE_SKIP_VERIFY, intended only for DEV
				// environments with a self-signed Caddy certificate (see the
				// comment on OIDCConfig.InsecureSkipVerify).
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	discoveryCtx := ctx
	if httpClient != nil {
		discoveryCtx = oidc.ClientContext(ctx, httpClient)
	}

	provider, err := oidc.NewProvider(discoveryCtx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("could not resolve oidc provider %q: %w", cfg.IssuerURL, err)
	}

	return &oidcAuthenticator{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
		},
		adminGroup: cfg.AdminGroup,
		httpClient: httpClient,
	}, nil
}

// authCodeURL returns the Authentik authorize URL for a new login
// attempt, including PKCE (S256) and nonce.
func (o *oidcAuthenticator) authCodeURL(state, nonce, pkceVerifier string) string {
	return o.oauth2Cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pkceVerifier),
	)
}

// oidcClaims are the fields read from the ID token that are relevant to
// the app.
type oidcClaims struct {
	Subject string
	Email   string
	Groups  []string
}

// isAdmin reports whether c is a member of the configured admin group.
func (c oidcClaims) isAdmin(adminGroup string) bool {
	for _, g := range c.Groups {
		if g == adminGroup {
			return true
		}
	}
	return false
}

// exchange exchanges code for tokens, validates the ID token (issuer,
// audience, signature, expiry, nonce) and reads the claims.
func (o *oidcAuthenticator) exchange(ctx context.Context, code, pkceVerifier, nonce string) (oidcClaims, error) {
	ctx = o.insecureContext(ctx)

	token, err := o.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return oidcClaims{}, fmt.Errorf("could not exchange code for tokens: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return oidcClaims{}, fmt.Errorf("response does not contain an id_token")
	}

	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return oidcClaims{}, fmt.Errorf("could not validate id token: %w", err)
	}
	if idToken.Nonce != nonce {
		return oidcClaims{}, fmt.Errorf("id token nonce does not match the login attempt")
	}

	var raw struct {
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&raw); err != nil {
		return oidcClaims{}, fmt.Errorf("could not read id token claims: %w", err)
	}

	return oidcClaims{Subject: idToken.Subject, Email: raw.Email, Groups: raw.Groups}, nil
}

// endSessionURL returns Authentik's RP-initiated logout URL (if discovery
// advertises an end_session_endpoint) with client_id as a parameter — ""
// if Authentik has no such endpoint, in which case /abmelden just
// redirects to /anmelden.
func (o *oidcAuthenticator) endSessionURL() string {
	var d struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := o.provider.Claims(&d); err != nil || d.EndSessionEndpoint == "" {
		return ""
	}

	u, err := url.Parse(d.EndSessionEndpoint)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", o.oauth2Cfg.ClientID)
	u.RawQuery = q.Encode()
	return u.String()
}
