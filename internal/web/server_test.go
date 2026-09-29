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

// testPublicHost is the Host header a reverse proxy in front of the
// server would pass through unchanged in production (see middleware.go
// requireHost) — tests connect via the actually bound loopback address
// (server.go bind()), but set the Host header to this fixed, public
// name, just like the proxy would.
const testPublicHost = "dmarc.example.test"

// testRedirectURL is the OIDC redirect URL matching testPublicHost — it
// also determines Server.allowedHost (see server.go New()). It's
// deliberately unresolvable (no real DNS entry) — tests that need to
// follow the redirect there rewrite the host to srv.Addr() (see
// beginFakeOIDCLogin) instead of actually resolving it.
const testRedirectURL = "http://" + testPublicHost + "/anmelden/callback"

func testDeps() Dependencies {
	return Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{stats: analysis.Statistics{}}},
	}
}

// testBuild is the build information of all test servers — see
// TestLayout_HeaderShowsVersionAndCommit.
var testBuild = BuildInfo{Version: "v9.9.9-test", Commit: "0123456789abcdef0123456789abcdef01234567"}

func testOIDCOptions(issuer string) Options {
	return Options{
		Addr:  "127.0.0.1:0",
		Build: testBuild,
		OIDC: OIDCConfig{
			IssuerURL:    issuer,
			ClientID:     testOIDCClientID,
			ClientSecret: "test-secret",
			RedirectURL:  testRedirectURL,
			AdminGroup:   "dmarc-admins",
		},
	}
}

// newRequest builds a request with context and sets the Host header to
// testPublicHost — just like a reverse proxy in front of the server
// would (TCP connection to the internally bound address, Host header
// carries the public name, see requireHost in middleware.go).
func newRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, target, body)
	require.NoError(t, err)
	req.Host = testPublicHost
	return req
}

// httpGet builds a GET request with context instead of the contextless
// http.Get/(*http.Client).Get convenience functions (linter: noctx).
func httpGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	resp, err := client.Do(newRequest(t, http.MethodGet, url, nil))
	require.NoError(t, err)
	return resp
}

// testCookieJar is a deliberately simplified http.CookieJar for tests:
// net/http/cookiejar would silently drop a Secure cookie (see auth.go
// setSessionCookie) when reading it back for this package's "http://"
// test server URLs (RFC 6265, correctly implemented by the standard
// library) — this fake deliberately ignores Secure/Domain/Path; it only
// serves to pass cookies along between the requests of ONE test client.
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

// newTestServer builds and starts a server with a local fake OIDC
// provider (see fakeoidc_test.go) on a real, random loopback port —
// deliberately not httptest.NewServer: Server.Start already binds a real
// net.Listener itself (see server.go).
func newTestServer(t *testing.T) (*Server, *fakeOIDCProvider) {
	t.Helper()

	provider := newFakeOIDCProvider(t)
	srv, err := New(context.Background(), testDeps(), testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv, provider
}

// beginFakeOIDCLogin runs through /anmelden and the fake provider up to
// just before the callback to our server: returns a client with the
// pending-login cookie in the jar, plus the callback URL rewritten to
// srv.Addr() (the provider actually redirects to testRedirectURL, which
// isn't resolvable — see the testRedirectURL documentation).
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

// loginViaFakeOIDC runs through the full login flow and returns a client
// whose cookie jar carries the resulting session for all following
// requests to srv.
func loginViaFakeOIDC(t *testing.T, srv *Server) *http.Client {
	t.Helper()

	client, callbackURL := beginFakeOIDCLogin(t, srv)

	resp := httpGet(t, client, callbackURL)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "a successful login should redirect to /")

	jar, _ := client.Jar.(*testCookieJar)
	require.NotEmpty(t, jar.cookiesNamed(sessionCookieName), "a successful login should create a session")

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
	opts.OIDC.RedirectURL = "not-complete"

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
	require.Empty(t, jar.cookiesNamed(sessionCookieName), "no session may be created without the admin group")

	// Explanatory HTML page instead of bare plaintext (verified
	// 2026-09-19: before this, apart from that one sentence, the page was
	// empty).
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Zugriff verweigert")
	require.Contains(t, string(body), "normal@example.com")
	require.Contains(t, string(body), `href="/anmelden"`)
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

// TestServer_HealthEndpoint_IgnoresHostHeader is a regression test for a
// bug found via "docker run": Docker's HEALTHCHECK (cmd_healthcheck.go)
// connects within the container via "127.0.0.1:<port>" — the Host
// header never carries the public hostname from DMARC_OIDC_REDIRECT_URL
// in that case. If /gesund were behind requireHost, the container would
// be permanently "unhealthy" (see the routes.go comment).
func TestServer_HealthEndpoint_IgnoresHostHeader(t *testing.T) {
	srv, _ := newTestServer(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+srv.Addr()+"/gesund", nil)
	require.NoError(t, err)
	req.Host = "127.0.0.1:12345" // like the in-container Docker healthcheck, not testPublicHost

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
