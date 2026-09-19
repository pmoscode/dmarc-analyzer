package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newSelfSignedDiscoveryServer liefert einen TLS-Testserver mit
// selbstsigniertem Zertifikat (httptest.NewTLSServer), der nur das
// Discovery-Dokument bedient — für newOIDCAuthenticator reicht das, JWKS
// wird erst bei einer echten Anmeldung gebraucht (siehe fakeoidc_test.go
// für den vollständigen Flow gegen einen unverschlüsselten Fake-Provider).
func newSelfSignedDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewUnstartedServer(mux)
	server.StartTLS()
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]any{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/authorize",
			"token_endpoint":                        server.URL + "/token",
			"jwks_uri":                              server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	return server
}

// TestNewOIDCAuthenticator_SelfSignedCert_FailsWithoutInsecureSkipVerify
// belegt den eigentlichen Grund für DMARC_OIDC_INSECURE_SKIP_VERIFY: gegen
// einen Issuer mit selbstsigniertem Zertifikat (z. B. Caddys "tls internal"
// im FS-BS-VPS-Setup-DEV) schlägt die Discovery ohne den Schalter mit einem
// TLS-Fehler fehl.
func TestNewOIDCAuthenticator_SelfSignedCert_FailsWithoutInsecureSkipVerify(t *testing.T) {
	server := newSelfSignedDiscoveryServer(t)

	_, err := newOIDCAuthenticator(context.Background(), OIDCConfig{
		IssuerURL: server.URL,
		ClientID:  testOIDCClientID,
	})

	require.Error(t, err)
}

// TestNewOIDCAuthenticator_SelfSignedCert_SucceedsWithInsecureSkipVerify
// prüft, dass InsecureSkipVerify die TLS-Prüfung tatsächlich abschaltet —
// derselbe Server wie oben, diesmal mit gesetztem Schalter.
func TestNewOIDCAuthenticator_SelfSignedCert_SucceedsWithInsecureSkipVerify(t *testing.T) {
	server := newSelfSignedDiscoveryServer(t)

	auth, err := newOIDCAuthenticator(context.Background(), OIDCConfig{
		IssuerURL:          server.URL,
		ClientID:           testOIDCClientID,
		InsecureSkipVerify: true,
	})

	require.NoError(t, err)
	require.NotNil(t, auth)
	require.NotNil(t, auth.httpClient, "httpClient sollte gesetzt sein, damit spaetere JWKS-/Token-Calls denselben Client verwenden")
}
