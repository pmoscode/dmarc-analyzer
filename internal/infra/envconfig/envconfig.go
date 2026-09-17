// Package envconfig liest die gesamte Laufzeit-Konfiguration einmalig aus
// Umgebungsvariablen (12-factor, siehe README.md/docs/features/deployment.md)
// — IMAP-Zugangsdaten, Aufbewahrungsdauer/Sync-Intervall, Datenverzeichnis,
// Listen-Adresse und OIDC-Parameter für die Authentik-Anmeldung. Es gibt
// keine Laufzeit-Änderung und keine Persistenz dieser Werte (ersetzt die
// frühere OS-Schlüsselbund-/JSON-Datei-Konfiguration der Desktop-Ära) —
// eine geänderte Einstellung braucht einen Container-Neustart.
package envconfig

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/pmoscode/dmarc-analyzer/internal/domain/account"
)

// Config ist die vollständige, beim Start einmalig geladene Konfiguration.
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

// OIDC bündelt die Parameter für die Authentik-Anmeldung (Authorization
// Code Flow, siehe internal/web/handlers_login.go).
type OIDC struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// AdminGroup ist der Authentik-Gruppenname, den der "groups"-Claim des
	// ID-Tokens enthalten muss — fehlt er, wird der Zugriff verweigert.
	AdminGroup string
}

// Load liest und validiert alle Umgebungsvariablen. Bei fehlenden
// Pflichtangaben oder ungültigen Werten werden ALLE Probleme gesammelt
// zurückgegeben (nicht nur das erste) — ein Container-Betreiber soll nicht
// die Startschleife mehrfach durchlaufen müssen, um jede fehlende Variable
// einzeln zu entdecken.
func Load() (Config, error) {
	var errs []error
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s ist nicht gesetzt", key))
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
			errs = append(errs, fmt.Errorf("%s ist keine gültige ganze zahl: %q", key, v))
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
			errs = append(errs, fmt.Errorf("%s ist kein gültiger wahrheitswert (true/false/1/0): %q", key, v))
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
			IssuerURL:    req("DMARC_OIDC_ISSUER_URL"),
			ClientID:     req("DMARC_OIDC_CLIENT_ID"),
			ClientSecret: req("DMARC_OIDC_CLIENT_SECRET"),
			RedirectURL:  req("DMARC_OIDC_REDIRECT_URL"),
			AdminGroup:   req("DMARC_OIDC_ADMIN_GROUP"),
		},
	}

	password := req("DMARC_IMAP_PASSWORD")
	cfg.IMAPSecret = account.NewSecretFromString(password)

	if cfg.IMAPPort < 1 || cfg.IMAPPort > 65535 {
		errs = append(errs, fmt.Errorf("DMARC_IMAP_PORT ist ungültig: %d", cfg.IMAPPort))
	}
	if cfg.RetentionMonths < 0 {
		errs = append(errs, fmt.Errorf("DMARC_RETENTION_MONTHS darf nicht negativ sein: %d", cfg.RetentionMonths))
	}
	if cfg.SyncIntervalMinutes < 0 {
		errs = append(errs, fmt.Errorf("DMARC_SYNC_INTERVAL_MINUTES darf nicht negativ sein: %d", cfg.SyncIntervalMinutes))
	}
	if cfg.OIDC.RedirectURL != "" {
		u, err := url.Parse(cfg.OIDC.RedirectURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Errorf("DMARC_OIDC_REDIRECT_URL ist keine vollständige URL: %q", cfg.OIDC.RedirectURL))
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("konfiguration ungültig:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}
