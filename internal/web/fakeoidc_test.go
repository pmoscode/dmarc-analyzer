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

// fakeOIDCProvider ist ein minimaler, lokaler OIDC-Identity-Provider für
// Tests — ein echter Authentik-Server lässt sich in dieser
// Testumgebung nicht ansprechen. Deckt genau das ab, was oidc.go
// tatsächlich braucht: Discovery, JWKS, Authorization-Endpoint
// (überspringt jede echte Anmeldemaske und leitet sofort mit einem Code
// zurück) und Token-Endpoint (liefert ein echt signiertes ID-Token).
// Damit prüfen die Tests den tatsächlichen Code-Pfad in
// oidc.go/handlers_login.go (Discovery, JWKS-Signaturprüfung,
// Code-Austausch, Nonce-Abgleich), nicht nur eine mit Mocks verdeckte
// Handler-Logik.
type fakeOIDCProvider struct {
	server     *httptest.Server
	signingKey *rsa.PrivateKey
	clientID   string

	mu sync.Mutex
	// nextGroups/nextSubject/nextEmail steuern den Inhalt des als
	// nächstes ausgestellten ID-Tokens — je Testfall änderbar.
	nextGroups  []string
	nextSubject string
	nextEmail   string
	lastNonce   string
}

// testOIDCClientID ist die Client-ID, die alle Tests dieses Pakets sowohl
// beim Fake-Provider als auch in Options.OIDC.ClientID verwenden — ein
// Parameter dafür wäre hier reine Formsache, da nie ein zweiter Wert
// vorkommt.
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

// setClaims legt fest, welche Gruppen/Subject/E-Mail das nächste über
// /token ausgestellte ID-Token trägt.
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

// handleAuthorize überspringt jede echte Nutzerinteraktion: merkt sich
// nonce und leitet sofort mit einem festen Code + dem übergebenen state
// zum redirect_uri zurück, wie es ein Browser nach erfolgreicher
// Anmeldung bei Authentik täte.
func (p *fakeOIDCProvider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	p.mu.Lock()
	p.lastNonce = q.Get("nonce")
	p.mu.Unlock()

	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "ungültiger redirect_uri", http.StatusBadRequest)
		return
	}
	rq := redirect.Query()
	rq.Set("code", "fake-code")
	rq.Set("state", q.Get("state"))
	redirect.RawQuery = rq.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// handleToken stellt unbedingt ein frisch signiertes ID-Token aus — ohne
// den Code oder Client-Zugangsdaten zu prüfen: dieses Fake testet die
// Client-Seite (oidc.go), nicht die Korrektheit eines echten
// Token-Endpoints.
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
