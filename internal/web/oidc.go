package web

import (
	"context"
	"fmt"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig sind die Parameter für die Authentik-Anmeldung (siehe
// internal/infra/envconfig.OIDC) — als eigener Typ hier, damit dieses
// Paket nicht von internal/infra/envconfig abhängen muss (AGENTS.md: web
// kennt nur eigene Typen/Ports, keine konkreten Infra-Pakete).
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AdminGroup ist der Authentik-Gruppenname, den der "groups"-Claim des
	// ID-Tokens enthalten muss.
	AdminGroup string
}

// oidcAuthenticator kapselt die Discovery gegen Authentik sowie den
// Austausch/die Prüfung von Tokens (Authorization Code Flow + PKCE,
// siehe handlers_login.go).
type oidcAuthenticator struct {
	provider   *oidc.Provider
	verifier   *oidc.IDTokenVerifier
	oauth2Cfg  oauth2.Config
	adminGroup string
}

// newOIDCAuthenticator lädt das Discovery-Dokument von cfg.IssuerURL —
// schlägt das fehl (Authentik nicht erreichbar, falsche Issuer-URL),
// meldet der Aufrufer das als klaren Startfehler statt eines defekten
// Logins zur Laufzeit.
func newOIDCAuthenticator(ctx context.Context, cfg OIDCConfig) (*oidcAuthenticator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc-provider %q konnte nicht ermittelt werden: %w", cfg.IssuerURL, err)
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
	}, nil
}

// authCodeURL liefert die Authentik-Authorize-URL für einen neuen
// Anmeldevorgang, inklusive PKCE (S256) und Nonce.
func (o *oidcAuthenticator) authCodeURL(state, nonce, pkceVerifier string) string {
	return o.oauth2Cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pkceVerifier),
	)
}

// oidcClaims sind die aus dem ID-Token gelesenen, für die App relevanten
// Felder.
type oidcClaims struct {
	Subject string
	Email   string
	Groups  []string
}

// isAdmin meldet, ob c Mitglied der konfigurierten Admin-Gruppe ist.
func (c oidcClaims) isAdmin(adminGroup string) bool {
	for _, g := range c.Groups {
		if g == adminGroup {
			return true
		}
	}
	return false
}

// exchange tauscht code gegen Tokens, validiert das ID-Token (Issuer,
// Audience, Signatur, Ablauf, Nonce) und liest die Claims.
func (o *oidcAuthenticator) exchange(ctx context.Context, code, pkceVerifier, nonce string) (oidcClaims, error) {
	token, err := o.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return oidcClaims{}, fmt.Errorf("code konnte nicht gegen tokens getauscht werden: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return oidcClaims{}, fmt.Errorf("antwort enthält kein id_token")
	}

	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return oidcClaims{}, fmt.Errorf("id-token konnte nicht validiert werden: %w", err)
	}
	if idToken.Nonce != nonce {
		return oidcClaims{}, fmt.Errorf("id-token-nonce stimmt nicht mit dem anmeldevorgang überein")
	}

	var raw struct {
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&raw); err != nil {
		return oidcClaims{}, fmt.Errorf("id-token-claims konnten nicht gelesen werden: %w", err)
	}

	return oidcClaims{Subject: idToken.Subject, Email: raw.Email, Groups: raw.Groups}, nil
}

// endSessionURL liefert Authentiks RP-initiated-Logout-URL (falls die
// Discovery einen end_session_endpoint bekanntgibt) mit client_id als
// Parameter — "" wenn Authentik keinen solchen Endpunkt hat, dann leitet
// /abmelden nur auf /anmelden um.
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
