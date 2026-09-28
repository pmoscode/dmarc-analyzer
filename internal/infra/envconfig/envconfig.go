// Package envconfig reads the entire runtime configuration once from
// environment variables (12-factor, see README.md/docs/features/deployment.md)
// — IMAP credentials, retention duration/sync interval, data directory,
// listen address, and OIDC parameters for the Authentik login. There is no
// runtime change and no persistence of these values (replaces the earlier
// OS keychain/JSON file configuration from the desktop era) — a changed
// setting requires a container restart.
package envconfig

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// Config is the complete configuration, loaded once at startup.
type Config struct {
	IMAPHost    string
	IMAPPort    int
	IMAPUser    string
	IMAPSecret  account.Secret
	IMAPMailbox string
	IMAPTLS     bool

	RetentionMonths     int
	SyncIntervalMinutes int

	DataDir    string
	ListenAddr string
	DevMode    bool

	OIDC OIDC
}

// OIDC bundles the parameters for the Authentik login (Authorization Code
// Flow, see internal/web/handlers_login.go).
type OIDC struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AdminGroup is the Authentik group name that the ID token's "groups"
	// claim must contain — if missing, access is denied.
	AdminGroup string
	// InsecureSkipVerify disables TLS certificate verification for all HTTP
	// calls against the OIDC issuer (discovery, JWKS, token exchange). Only
	// intended for development environments with a self-signed certificate
	// (e.g. Caddy's "tls internal" in the FS-BS-VPS setup) — never set this
	// in production, or token exchange and ID token validation are
	// unprotected against a man-in-the-middle.
	InsecureSkipVerify bool
}

// Load reads and validates all environment variables. If required values
// are missing or values are invalid, ALL problems are collected and
// returned (not just the first one) — a container operator shouldn't have
// to go through the startup loop multiple times to discover each missing
// variable one by one.
func Load() (Config, error) {
	var errs []error
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is not set", key))
		}
		return v
	}
	optInt := func(key string, def int) int {
		v := os.Getenv(key)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s is not a valid integer: %q", key, v))
			return def
		}
		return n
	}
	optBool := func(key string, def bool) bool {
		v := os.Getenv(key)
		if v == "" {
			return def
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s is not a valid boolean (true/false/1/0): %q", key, v))
			return def
		}
		return b
	}
	optStr := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		IMAPHost:    req("DMARC_IMAP_HOST"),
		IMAPPort:    optInt("DMARC_IMAP_PORT", 993),
		IMAPUser:    req("DMARC_IMAP_USER"),
		IMAPMailbox: optStr("DMARC_IMAP_MAILBOX", "INBOX"),
		IMAPTLS:     optBool("DMARC_IMAP_TLS", true),

		RetentionMonths:     optInt("DMARC_RETENTION_MONTHS", 24),
		SyncIntervalMinutes: optInt("DMARC_SYNC_INTERVAL_MINUTES", 60),

		DataDir:    optStr("DMARC_DATA_DIR", "/data"),
		ListenAddr: optStr("DMARC_LISTEN_ADDR", ":8080"),
		DevMode:    optBool("DMARC_DEV_MODE", false),

		OIDC: OIDC{
			IssuerURL:          req("DMARC_OIDC_ISSUER_URL"),
			ClientID:           req("DMARC_OIDC_CLIENT_ID"),
			ClientSecret:       req("DMARC_OIDC_CLIENT_SECRET"),
			RedirectURL:        req("DMARC_OIDC_REDIRECT_URL"),
			AdminGroup:         req("DMARC_OIDC_ADMIN_GROUP"),
			InsecureSkipVerify: optBool("DMARC_OIDC_INSECURE_SKIP_VERIFY", false),
		},
	}

	password := req("DMARC_IMAP_PASSWORD")
	cfg.IMAPSecret = account.NewSecretFromString(password)

	if cfg.IMAPPort < 1 || cfg.IMAPPort > 65535 {
		errs = append(errs, fmt.Errorf("DMARC_IMAP_PORT is invalid: %d", cfg.IMAPPort))
	}
	if cfg.RetentionMonths < 0 {
		errs = append(errs, fmt.Errorf("DMARC_RETENTION_MONTHS must not be negative: %d", cfg.RetentionMonths))
	}
	if cfg.SyncIntervalMinutes < 0 {
		errs = append(errs, fmt.Errorf("DMARC_SYNC_INTERVAL_MINUTES must not be negative: %d", cfg.SyncIntervalMinutes))
	}
	if cfg.OIDC.RedirectURL != "" {
		u, err := url.Parse(cfg.OIDC.RedirectURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Errorf("DMARC_OIDC_REDIRECT_URL is not a complete URL: %q", cfg.OIDC.RedirectURL))
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}
