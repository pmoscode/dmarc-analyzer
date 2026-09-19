package envconfig_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/infra/envconfig"
)

// setValidEnv setzt alle Pflichtvariablen auf gültige Werte — Tests
// überschreiben gezielt einzelne, um ein bestimmtes Verhalten zu prüfen.
func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DMARC_IMAP_HOST", "imap.example.com")
	t.Setenv("DMARC_IMAP_USER", "dmarc@example.com")
	t.Setenv("DMARC_IMAP_PASSWORD", "geheim123")
	t.Setenv("DMARC_OIDC_ISSUER_URL", "https://authentik.example.com/application/o/dmarc/")
	t.Setenv("DMARC_OIDC_CLIENT_ID", "dmarc-analyzer")
	t.Setenv("DMARC_OIDC_CLIENT_SECRET", "oidc-secret")
	t.Setenv("DMARC_OIDC_REDIRECT_URL", "https://dmarc.example.com/anmelden/callback")
	t.Setenv("DMARC_OIDC_ADMIN_GROUP", "dmarc-admins")
}

func TestLoad_ValidEnv_AppliesDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := envconfig.Load()

	require.NoError(t, err)
	require.Equal(t, "imap.example.com", cfg.IMAPHost)
	require.Equal(t, 993, cfg.IMAPPort)
	require.Equal(t, "INBOX", cfg.IMAPMailbox)
	require.True(t, cfg.IMAPTLS)
	require.Equal(t, []byte("geheim123"), cfg.IMAPSecret.Expose())
	require.Equal(t, 24, cfg.RetentionMonths)
	require.Equal(t, 60, cfg.SyncIntervalMinutes)
	require.Equal(t, "/data", cfg.DataDir)
	require.Equal(t, ":8080", cfg.ListenAddr)
	require.False(t, cfg.DevMode)
	require.Equal(t, "dmarc-admins", cfg.OIDC.AdminGroup)
	require.False(t, cfg.OIDC.InsecureSkipVerify)
}

func TestLoad_InsecureSkipVerify_Enabled(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_OIDC_INSECURE_SKIP_VERIFY", "true")

	cfg, err := envconfig.Load()

	require.NoError(t, err)
	require.True(t, cfg.OIDC.InsecureSkipVerify)
}

func TestLoad_OverridesDefaults(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_IMAP_PORT", "143")
	t.Setenv("DMARC_IMAP_MAILBOX", "DMARC")
	t.Setenv("DMARC_IMAP_TLS", "false")
	t.Setenv("DMARC_RETENTION_MONTHS", "12")
	t.Setenv("DMARC_SYNC_INTERVAL_MINUTES", "15")
	t.Setenv("DMARC_DATA_DIR", "/var/lib/dmarc")
	t.Setenv("DMARC_LISTEN_ADDR", "0.0.0.0:9090")
	t.Setenv("DMARC_DEV_MODE", "true")

	cfg, err := envconfig.Load()

	require.NoError(t, err)
	require.Equal(t, 143, cfg.IMAPPort)
	require.Equal(t, "DMARC", cfg.IMAPMailbox)
	require.False(t, cfg.IMAPTLS)
	require.Equal(t, 12, cfg.RetentionMonths)
	require.Equal(t, 15, cfg.SyncIntervalMinutes)
	require.Equal(t, "/var/lib/dmarc", cfg.DataDir)
	require.Equal(t, "0.0.0.0:9090", cfg.ListenAddr)
	require.True(t, cfg.DevMode)
}

func TestLoad_MissingRequiredVars_ReportsAllOfThem(t *testing.T) {
	// Bewusst kein setValidEnv() — alle Pflichtvariablen fehlen.
	_, err := envconfig.Load()

	require.Error(t, err)
	for _, want := range []string{
		"DMARC_IMAP_HOST", "DMARC_IMAP_USER", "DMARC_IMAP_PASSWORD",
		"DMARC_OIDC_ISSUER_URL", "DMARC_OIDC_CLIENT_ID", "DMARC_OIDC_CLIENT_SECRET",
		"DMARC_OIDC_REDIRECT_URL", "DMARC_OIDC_ADMIN_GROUP",
	} {
		require.Contains(t, err.Error(), want, "fehlermeldung sollte %s erwähnen", want)
	}
}

func TestLoad_InvalidPort_ReturnsError(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_IMAP_PORT", "not-a-number")

	_, err := envconfig.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DMARC_IMAP_PORT")
}

func TestLoad_PortOutOfRange_ReturnsError(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_IMAP_PORT", "70000")

	_, err := envconfig.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DMARC_IMAP_PORT")
}

func TestLoad_NegativeRetention_ReturnsError(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_RETENTION_MONTHS", "-1")

	_, err := envconfig.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DMARC_RETENTION_MONTHS")
}

func TestLoad_InvalidBool_ReturnsError(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_IMAP_TLS", "vielleicht")

	_, err := envconfig.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DMARC_IMAP_TLS")
}

func TestLoad_RedirectURLWithoutScheme_ReturnsError(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DMARC_OIDC_REDIRECT_URL", "dmarc.example.com/anmelden/callback")

	_, err := envconfig.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DMARC_OIDC_REDIRECT_URL")
}
