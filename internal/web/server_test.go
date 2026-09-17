package web

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
)

// isolateConfigDir zeigt paths.ConfigDir() (über os.UserConfigDir()) auf
// ein temporäres Verzeichnis — dieselbe Absicherung wie
// internal/platform/paths/paths_test.go, damit Tests nicht in echte
// Konfigurationsordner schreiben.
func isolateConfigDir(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("APPDATA", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
}

func testDeps() Dependencies {
	return Dependencies{
		Statistics: &statistics.UseCase{Repository: &fakeRepository{stats: analysis.Statistics{}}},
	}
}

// httpGet baut eine GET-Anfrage mit Kontext statt der kontextlosen
// http.Get/(*http.Client).Get-Bequemlichkeitsfunktionen (linter: noctx) —
// hier stets mit context.Background(), weil die Testfälle keine
// Anfrage-spezifische Fristen/Abbrüche brauchen.
func httpGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// newTestServer baut und startet einen Server auf einem echten,
// zufälligen Loopback-Port — bewusst kein httptest.NewServer: Server.Start
// bindet bereits selbst einen echten net.Listener (siehe server.go), und
// gerade das Host-Header-/Adress-Verhalten soll gegen eine echte,
// gebundene Adresse geprüft werden, nicht gegen ein Test-Double.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	isolateConfigDir(t)

	srv, err := New(testDeps(), Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	_, err = srv.Start(context.Background())
	require.NoError(t, err)
	return srv
}

func TestNew_RejectsNonLoopbackAddress(t *testing.T) {
	isolateConfigDir(t)

	_, err := New(testDeps(), Options{Addr: "0.0.0.0:0"})
	require.Error(t, err)
}

func TestServer_Start_WritesInstanceFileWithSecret(t *testing.T) {
	srv := newTestServer(t)

	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	instancePath := filepath.Join(dir, "dmarc-analyzer", "instance.json")

	data, err := os.ReadFile(instancePath)
	require.NoError(t, err)
	require.Contains(t, string(data), srv.auth.instanceSecret)
}

func TestServer_Shutdown_RemovesInstanceFile(t *testing.T) {
	srv := newTestServer(t)

	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	instancePath := filepath.Join(dir, "dmarc-analyzer", "instance.json")

	require.NoError(t, srv.Shutdown(context.Background()))
	_, err = os.Stat(instancePath)
	require.True(t, os.IsNotExist(err))
}

func TestServer_LoginFlow_ValidCode_GrantsSessionAndAccess(t *testing.T) {
	srv := newTestServer(t)

	code, err := srv.auth.issueCode()
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	loginURL := "http://" + srv.Addr() + "/anmelden?code=" + code
	resp := httpGet(t, client, loginURL)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Redirect + Folgeanfrage muss auf der Übersicht landen")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Übersicht")
}

func TestServer_WithoutSession_Returns401(t *testing.T) {
	srv := newTestServer(t)

	resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+"/")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestServer_Login_InvalidCode_Returns401(t *testing.T) {
	srv := newTestServer(t)

	resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+"/anmelden?code=falsch")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestServer_ForeignHostHeader_Rejected(t *testing.T) {
	srv := newTestServer(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+srv.Addr()+"/", nil)
	require.NoError(t, err)
	req.Host = "evil.example.com"

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusMisdirectedRequest, resp.StatusCode)
}

func TestServer_StaticAssets_ServedWithoutSession(t *testing.T) {
	srv := newTestServer(t)

	resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+"/static/app.css")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, body)
}

func TestServer_ChartEndpoints_RequireSession(t *testing.T) {
	srv := newTestServer(t)

	for _, path := range []string{"/api/diagramme/verlauf", "/api/diagramme/heatmap", "/api/diagramme/quellen", "/api/diagramme/disposition"} {
		resp := httpGet(t, http.DefaultClient, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
		_ = resp.Body.Close()
	}
}
