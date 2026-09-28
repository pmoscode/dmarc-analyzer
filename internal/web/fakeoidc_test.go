package web

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
)

// fakeOIDCProvider is a minimal, local OIDC identity provider for
// tests — a real Authentik server can't be reached in this test
// environment. Covers exactly what oidc.go actually needs: discovery,
// JWKS, an authorization endpoint (skips any real login screen and
// redirects immediately with a code) and a token endpoint (returns a
// genuinely signed ID token). This lets the tests check the actual code
// path in oidc.go/handlers_login.go (discovery, JWKS signature
// verification, code exchange, nonce matching), not just handler logic
// hidden behind mocks.
type fakeOIDCProvider struct {
	server     *httptest.Server
	signingKey *rsa.PrivateKey
	clientID   string

	mu sync.Mutex
	// nextGroups/nextSubject/nextEmail control the content of the next
	// issued ID token — changeable per test case.
	nextGroups  []string
	nextSubject string
	nextEmail   string
	lastNonce   string
}

// testOIDCClientID is the client ID all tests in this package use both
// with the fake provider and in Options.OIDC.ClientID — a parameter for
// this would be pure formality here, since a second value never occurs.
const testOIDCClientID = "test-client"

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p := &fakeOIDCProvider{signingKey: key, clientID: testOIDCClientID, nextSubject: "test-subject", nextEmail: "admin@example.com"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("/jwks", p.handleJWKS)
	mux.HandleFunc("/authorize", p.handleAuthorize)
	mux.HandleFunc("/token", p.handleToken)
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

func (p *fakeOIDCProvider) issuer() string { return p.server.URL }

// setClaims sets which groups/subject/email the next ID token issued via
// /token carries.
func (p *fakeOIDCProvider) setClaims(subject, email string, groups []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextSubject, p.nextEmail, p.nextGroups = subject, email, groups
}

func (p *fakeOIDCProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                p.issuer(),
		"authorization_endpoint":                p.issuer() + "/authorize",
		"token_endpoint":                        p.issuer() + "/token",
		"jwks_uri":                              p.issuer() + "/jwks",
		"end_session_endpoint":                  p.issuer() + "/endsession",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (p *fakeOIDCProvider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &p.signingKey.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig",
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

// handleAuthorize skips any real user interaction: remembers the nonce
// and redirects immediately with a fixed code + the given state to the
// redirect_uri, as a browser would after a successful Authentik login.
func (p *fakeOIDCProvider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	p.mu.Lock()
	p.lastNonce = q.Get("nonce")
	p.mu.Unlock()

	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	rq := redirect.Query()
	rq.Set("code", "fake-code")
	rq.Set("state", q.Get("state"))
	redirect.RawQuery = rq.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// handleToken unconditionally issues a freshly signed ID token — without
// checking the code or client credentials: this fake tests the client
// side (oidc.go), not the correctness of a real token endpoint.
func (p *fakeOIDCProvider) handleToken(w http.ResponseWriter, _ *http.Request) {
	idToken, err := p.signIDToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"id_token":     idToken,
		"expires_in":   3600,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (p *fakeOIDCProvider) signIDToken() (string, error) {
	p.mu.Lock()
	claims := map[string]any{
		"iss":    p.issuer(),
		"sub":    p.nextSubject,
		"aud":    p.clientID,
		"exp":    time.Now().Add(5 * time.Minute).Unix(),
		"iat":    time.Now().Unix(),
		"nonce":  p.lastNonce,
		"email":  p.nextEmail,
		"groups": p.nextGroups,
	}
	p.mu.Unlock()

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.signingKey},
		(&jose.SignerOptions{}).WithHeader("kid", "test-key"),
	)
	if err != nil {
		return "", err
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return obj.CompactSerialize()
}
