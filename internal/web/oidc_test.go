package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newSelfSignedDiscoveryServer returns a TLS test server with a
// self-signed certificate (httptest.NewTLSServer) that only serves the
// discovery document — that's enough for newOIDCAuthenticator, JWKS is
// only needed for an actual login (see fakeoidc_test.go for the full
// flow against an unencrypted fake provider).
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
// demonstrates the actual reason for DMARC_OIDC_INSECURE_SKIP_VERIFY:
// against an issuer with a self-signed certificate (e.g. Caddy's "tls
// internal" in the FS-BS-VPS-Setup-DEV) discovery fails with a TLS error
// without the flag.
func TestNewOIDCAuthenticator_SelfSignedCert_FailsWithoutInsecureSkipVerify(t *testing.T) {
	server := newSelfSignedDiscoveryServer(t)

	_, err := newOIDCAuthenticator(context.Background(), OIDCConfig{
		IssuerURL: server.URL,
		ClientID:  testOIDCClientID,
	})

	require.Error(t, err)
}

// TestNewOIDCAuthenticator_SelfSignedCert_SucceedsWithInsecureSkipVerify
// verifies that InsecureSkipVerify actually disables the TLS check — the
// same server as above, this time with the flag set.
func TestNewOIDCAuthenticator_SelfSignedCert_SucceedsWithInsecureSkipVerify(t *testing.T) {
	server := newSelfSignedDiscoveryServer(t)

	auth, err := newOIDCAuthenticator(context.Background(), OIDCConfig{
		IssuerURL:          server.URL,
		ClientID:           testOIDCClientID,
		InsecureSkipVerify: true,
	})

	require.NoError(t, err)
	require.NotNil(t, auth)
	require.NotNil(t, auth.httpClient, "httpClient should be set so later JWKS/token calls use the same client")
}
