package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
)

// testPublicHost ist der Host-Header, den ein vorgeschalteter Reverse-Proxy
// in Produktion unverändert durchreichen würde (siehe middleware.go
// requireHost) — Tests verbinden sich zwar über die tatsächlich gebundene
// Loopback-Adresse (server.go bind()), setzen den Host-Header aber auf
// diesen festen, öffentlichen Namen, genau wie es der Proxy täte.
const testPublicHost = "dmarc.example.test"

// testRedirectURL ist die zu testPublicHost passende OIDC-Redirect-URL —
// bestimmt zugleich Server.allowedHost (siehe server.go New()). Sie ist
// absichtlich nicht auflösbar (kein echter DNS-Eintrag) — Tests, die dem
// Redirect dorthin folgen müssen, schreiben den Host auf srv.Addr() um
// (siehe beginFakeOIDCLogin), statt tatsächlich danach aufzulösen.
const testRedirectURL = "http://" + testPublicHost + "/anmelden/callback"

func testDeps() Dependencies {
	return Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{stats: analysis.Statistics{}}},
	}
}

func testOIDCOptions(issuer string) Options {
	return Options{
		Addr: "127.0.0.1:0",
		OIDC: OIDCConfig{
			IssuerURL:    issuer,
			ClientID:     testOIDCClientID,
			ClientSecret: "test-secret",
			RedirectURL:  testRedirectURL,
			AdminGroup:   "dmarc-admins",
		},
	}
}

// newRequest baut eine Anfrage mit Kontext und setzt den Host-Header auf
// testPublicHost — genau wie ein vorgeschalteter Reverse-Proxy es täte
// (TCP-Verbindung zur intern gebundenen Adresse, Host-Header trägt den
// öffentlichen Namen, siehe requireHost in middleware.go).
func newRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, target, body)
	require.NoError(t, err)
	req.Host = testPublicHost
	return req
}

// httpGet baut eine GET-Anfrage mit Kontext statt der kontextlosen
// http.Get/(*http.Client).Get-Bequemlichkeitsfunktionen (linter: noctx).
func httpGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	resp, err := client.Do(newRequest(t, http.MethodGet, url, nil))
	require.NoError(t, err)
	return resp
}

// testCookieJar ist ein bewusst vereinfachter http.CookieJar für Tests:
// net/http/cookiejar würde ein Secure-Cookie (siehe auth.go
// setSessionCookie) für die "http://"-Testserver-URLs dieses Pakets beim
// Zurücklesen stillschweigend verwerfen (RFC 6265, von der Standardbibliothek
// korrekt umgesetzt) — dieses Fake ignoriert Secure/Domain/Path bewusst,
// es dient nur dazu, Cookies zwischen den Anfragen EINES Testclients
// weiterzureichen.
type testCookieJar struct {
	mu      sync.Mutex
	cookies map[string]*http.Cookie
}

func newSessionCapturingJar() *testCookieJar {
	return &testCookieJar{cookies: make(map[string]*http.Cookie)}
}

func (j *testCookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range cookies {
		if c.MaxAge < 0 {
			delete(j.cookies, c.Name)
			continue
		}
		cp := *c
		j.cookies[c.Name] = &cp
	}
}

func (j *testCookieJar) Cookies(*url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]*http.Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return out
}

func (j *testCookieJar) cookiesNamed(name string) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	if c, ok := j.cookies[name]; ok {
		return []*http.Cookie{c}
	}
	return nil
}

// newTestServer baut und startet einen Server mit einem lokalen
// Fake-OIDC-Provider (siehe fakeoidc_test.go) auf einem echten, zufälligen
// Loopback-Port — bewusst kein httptest.NewServer: Server.Start bindet
// bereits selbst einen echten net.Listener (siehe server.go).
func newTestServer(t *testing.T) (*Server, *fakeOIDCProvider) {
	t.Helper()

	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), testDeps(), testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv, provider
}

// beginFakeOIDCLogin durchläuft /anmelden und den Fake-Provider bis kurz
// vor dem Callback an unseren Server: liefert einen Client mit dem
// pending-login-Cookie im Jar sowie die auf srv.Addr() umgeschriebene
// Callback-URL (der Provider leitet eigentlich auf testRedirectURL um,
// die nicht auflösbar ist — siehe testRedirectURL-Dokumentation).
func beginFakeOIDCLogin(t *testing.T, srv *Server) (*http.Client, string) {
	t.Helper()

	client := &http.Client{
		Jar:           newSessionCapturingJar(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	startResp := httpGet(t, client, "http://"+srv.Addr()+"/anmelden")
	require.Equal(t, http.StatusSeeOther, startResp.StatusCode)
	authorizeURL := startResp.Header.Get("Location")
	require.NoError(t, startResp.Body.Close())

	authorizeResp := httpGet(t, client, authorizeURL)
	require.Equal(t, http.StatusFound, authorizeResp.StatusCode)
	callbackURL := authorizeResp.Header.Get("Location")
	require.NoError(t, authorizeResp.Body.Close())

	u, err := url.Parse(callbackURL)
	require.NoError(t, err)
	u.Host = srv.Addr()

	return client, u.String()
}

// loginViaFakeOIDC durchläuft den vollständigen Anmeldevorgang und liefert
// einen Client, dessen Cookie-Jar die entstandene Sitzung für alle
// folgenden Anfragen an srv mitträgt.
func loginViaFakeOIDC(t *testing.T, srv *Server) *http.Client {
	t.Helper()

	client, callbackURL := beginFakeOIDCLogin(t, srv)

	resp := httpGet(t, client, callbackURL)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "erfolgreiche anmeldung sollte auf / weiterleiten")

	jar, _ := client.Jar.(*testCookieJar)
	require.NotEmpty(t, jar.cookiesNamed(sessionCookieName), "erfolgreiche anmeldung sollte eine sitzung anlegen")

	return client
}

func TestNew_UnreachableIssuer_ReturnsError(t *testing.T) {
	opts := testOIDCOptions("http://127.0.0.1:1/nichts-hier")

	_, err := New(context.Background(), testDeps(), opts)

	require.Error(t, err)
}

func TestNew_InvalidRedirectURL_ReturnsError(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	opts := testOIDCOptions(provider.issuer())
	opts.OIDC.RedirectURL = "nicht-vollstaendig"

	_, err := New(context.Background(), testDeps(), opts)

	require.Error(t, err)
}

func TestServer_LoginFlow_ValidAdminGroup_GrantsSessionAndAccess(t *testing.T) {
	srv, provider := newTestServer(t)
	provider.setClaims("admin-subject", "admin@example.com", []string{"dmarc-admins"})

	client := loginViaFakeOIDC(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Übersicht")
}

func TestServer_LoginFlow_WithoutAdminGroup_Returns403AndNoSession(t *testing.T) {
	srv, provider := newTestServer(t)
	provider.setClaims("normal-subject", "normal@example.com", []string{"everyone"})

	client, callbackURL := beginFakeOIDCLogin(t, srv)

	resp := httpGet(t, client, callbackURL)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	jar, _ := client.Jar.(*testCookieJar)
	require.Empty(t, jar.cookiesNamed(sessionCookieName), "ohne admin-gruppe darf keine sitzung entstehen")
}

func TestServer_Login_InvalidState_Returns400(t *testing.T) {
	srv, _ := newTestServer(t)

	client := &http.Client{
		Jar:           newSessionCapturingJar(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	startResp := httpGet(t, client, "http://"+srv.Addr()+"/anmelden")
	require.Equal(t, http.StatusSeeOther, startResp.StatusCode)
	require.NoError(t, startResp.Body.Close())

	resp := httpGet(t, client, "http://"+srv.Addr()+"/anmelden/callback?code=irgendwas&state=falsch")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_WithoutSession_RedirectsToLogin(t *testing.T) {
	srv, _ := newTestServer(t)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp := httpGet(t, client, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/anmelden", resp.Header.Get("Location"))
}

func TestServer_ForeignHostHeader_Rejected(t *testing.T) {
	srv, _ := newTestServer(t)

	req := newRequest(t, http.MethodGet, "http://"+srv.Addr()+"/", nil)
	req.Host = "evil.example.com"

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusMisdirectedRequest, resp.StatusCode)
}

func TestServer_StaticAssets_ServedWithoutSession(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+"/static/app.css")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, body)
}

func TestServer_HealthEndpoint_ServedWithoutSession(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+"/gesund")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_HealthEndpoint_IgnoresHostHeader ist eine Regression für einen
// per "docker run" gefundenen Fehler: Dockers HEALTHCHECK
// (cmd_healthcheck.go) verbindet sich containerintern über
// "127.0.0.1:<port>" — der Host-Header trägt dabei nie den öffentlichen
// Hostnamen aus DMARC_OIDC_REDIRECT_URL. Läge /gesund hinter requireHost,
// wäre der Container dauerhaft "unhealthy" (siehe routes.go-Kommentar).
func TestServer_HealthEndpoint_IgnoresHostHeader(t *testing.T) {
	srv, _ := newTestServer(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+srv.Addr()+"/gesund", nil)
	require.NoError(t, err)
	req.Host = "127.0.0.1:12345" // wie beim containerinternen Docker-Healthcheck, nicht testPublicHost

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_ChartEndpoints_RedirectWithoutSession(t *testing.T) {
	srv, _ := newTestServer(t)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, path := range []string{"/api/diagramme/verlauf", "/api/diagramme/heatmap", "/api/diagramme/quellen", "/api/diagramme/disposition"} {
		resp := httpGet(t, client, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusSeeOther, resp.StatusCode, path)
		_ = resp.Body.Close()
	}
}
