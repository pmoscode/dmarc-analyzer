package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// sessionTransport injects the session cookie of a session created via
// authenticatedClient into every request — its own mechanism,
// independent of Secure/SameSite, instead of a net/http/cookiejar:
// cookiejar would silently drop a Secure cookie (see auth.go
// setSessionCookie) when reading it back for this package's
// "http://" test server URLs.
type sessionTransport struct {
	cookieValue string
}

func (t *sessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: t.cookieValue})
	return http.DefaultTransport.RoundTrip(req)
}

// authenticatedClient creates a session directly (without going through
// the real OIDC detour via the fake provider for every handler test —
// the login-specific tests in server_test.go/handlers_login_test.go
// check that separately) and returns a client that sends the session
// cookie with every request.
func authenticatedClient(t *testing.T, srv *Server) *http.Client {
	t.Helper()

	cookieValue, _, err := srv.auth.createSession("test-admin@example.com")
	require.NoError(t, err)

	return &http.Client{Transport: &sessionTransport{cookieValue: cookieValue}}
}

// csrfTokenFor returns the CSRF token of client's session (see
// authenticatedClient) — for form tests that need to pass through
// requireCSRF.
func csrfTokenFor(t *testing.T, srv *Server, client *http.Client) string {
	t.Helper()

	tr, ok := client.Transport.(*sessionTransport)
	require.True(t, ok, "client was not created via authenticatedClient")

	srv.auth.mu.Lock()
	defer srv.auth.mu.Unlock()
	s, ok := srv.auth.sessions[tr.cookieValue]
	require.True(t, ok, "session not found")
	return s.csrfToken
}

func newTestServerWithRepo(t *testing.T, repo *fakeRepository) *Server {
	t.Helper()

	provider := newFakeOIDCProvider(t)
	deps := Dependencies{Statistics: &statistics.UseCase{Repository: repo}}
	srv, err := New(context.Background(), deps, testOIDCOptions(provider.issuer()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	require.NoError(t, srv.Start(context.Background()))
	return srv
}

func TestHandleChartDailyVolume_ReturnsPointsWithDrilldownURLs(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{dailyVolumes: []analysis.DailyVolume{
		{Day: day, Pass: 10, Fail: 2},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/verlauf")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var got dailyVolumeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Days, 1)
	require.Equal(t, "01.09.", got.Days[0].Label)
	require.Equal(t, 10, got.Days[0].Pass)
	require.Equal(t, 2, got.Days[0].Fail)
	require.Equal(t, "/berichte?bis=2026-09-02&von=2026-09-01", got.Days[0].URL)
}

func TestHandleChartDailyVolume_EmptyData_ReturnsEmptyArrayNotNull(t *testing.T) {
	srv := newTestServerWithRepo(t, &fakeRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/verlauf")
	defer func() { _ = resp.Body.Close() }()

	var got dailyVolumeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.NotNil(t, got.Days)
	require.Empty(t, got.Days)
}

func TestHandleChartHeatmap_ReturnsCellsWithSourceLabelsAndDrilldownURLs(t *testing.T) {
	day1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	ip := mustSourceIP("203.0.113.1")

	repo := &fakeRepository{heatmap: analysis.Heatmap{
		Sources:      []report.SourceIP{ip},
		SourceLabels: []string{"Google Workspace"},
		Days:         []time.Time{day1, day2},
		Cells: [][]analysis.HeatmapCell{
			{
				{PassRate: 1, HasData: true, Total: 5},
				{HasData: false},
			},
		},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/heatmap")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got heatmapResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

	require.Equal(t, []string{"Google Workspace (203.0.113.1)"}, got.SourceLabels)
	require.Equal(t, []string{"01.09.", "02.09."}, got.DayLabels)
	require.Len(t, got.Cells, 2)

	require.Equal(t, "Google Workspace (203.0.113.1)", got.Cells[0].Y)
	require.Equal(t, "01.09.", got.Cells[0].X)
	require.True(t, got.Cells[0].HasData)
	require.InDelta(t, 1.0, got.Cells[0].PassRate, 0.0001)
	require.Equal(t, 5, got.Cells[0].Total)
	require.Equal(t, "/berichte?bis=2026-09-02&quelle=203.0.113.1&von=2026-09-01", got.Cells[0].URL)

	require.False(t, got.Cells[1].HasData)
}

func TestHandleChartHeatmap_DuplicateEnrichedLabels_StayDistinguishableByIP(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ipA := mustSourceIP("203.0.113.1")
	ipB := mustSourceIP("203.0.113.2")

	repo := &fakeRepository{heatmap: analysis.Heatmap{
		Sources:      []report.SourceIP{ipA, ipB},
		SourceLabels: []string{"Google Workspace", "Google Workspace"},
		Days:         []time.Time{day},
		Cells: [][]analysis.HeatmapCell{
			{{PassRate: 1, HasData: true, Total: 1}},
			{{PassRate: 0, HasData: true, Total: 1}},
		},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/heatmap")
	defer func() { _ = resp.Body.Close() }()

	var got heatmapResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

	require.Len(t, got.SourceLabels, 2)
	require.NotEqual(t, got.SourceLabels[0], got.SourceLabels[1],
		"two sources with the same detected service must not end up in the same chart row")
}

func TestHandleChartHeatmap_NoLabel_FallsBackToIP(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ip := mustSourceIP("203.0.113.9")

	repo := &fakeRepository{heatmap: analysis.Heatmap{
		Sources: []report.SourceIP{ip},
		Days:    []time.Time{day},
		Cells:   [][]analysis.HeatmapCell{{{PassRate: 0, HasData: false}}},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/heatmap")
	defer func() { _ = resp.Body.Close() }()

	var got heatmapResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, []string{"203.0.113.9"}, got.SourceLabels)
}

func TestHandleChartDailyVolume_ComputeError_Returns500NotPanic(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	// The error is only set after login: login itself redirects to "/"
	// and the test client follows the redirect — the overview also calls
	// Statistics.Dashboard and would already fail during login with
	// computeErr set from the start, not only at the request under test
	// here.
	client := authenticatedClient(t, srv)
	repo.computeErr = errTest

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/verlauf")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleChartDailyVolume_FilterParams_ReachRepository(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/verlauf?zeitraum=7&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastDailyVolumesQuery.Domain)
	require.WithinDuration(t,
		repo.lastDailyVolumesQuery.Period.Begin.AddDate(0, 0, 7),
		repo.lastDailyVolumesQuery.Period.End,
		time.Second)
}

func TestHandleChartDailyVolume_UnknownZeitraum_FallsBackToDefault(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/verlauf?zeitraum=999")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.WithinDuration(t,
		repo.lastDailyVolumesQuery.Period.Begin.AddDate(0, 0, defaultPeriodDays),
		repo.lastDailyVolumesQuery.Period.End,
		time.Second)
}

func TestHandleChartTopSources_ReturnsPointsWithLabelsAndDrilldownURLs(t *testing.T) {
	repo := &fakeRepository{topSources: []analysis.SourceVolume{
		{SourceIP: mustSourceIP("203.0.113.1"), Total: 100, PassRate: 0.9, Label: "Google Workspace"},
		{SourceIP: mustSourceIP("203.0.113.2"), Total: 10, PassRate: 0.1},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/quellen")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var got sourceVolumeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Sources, 2)

	require.Equal(t, "Google Workspace (203.0.113.1)", got.Sources[0].Label)
	require.Equal(t, 100, got.Sources[0].Total)
	require.InDelta(t, 0.9, got.Sources[0].PassRate, 0.0001)
	require.Contains(t, got.Sources[0].URL, "quelle=203.0.113.1")

	require.Equal(t, "203.0.113.2", got.Sources[1].Label)
}

func TestHandleChartTopSources_EmptyData_ReturnsEmptyArrayNotNull(t *testing.T) {
	srv := newTestServerWithRepo(t, &fakeRepository{})
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/quellen")
	defer func() { _ = resp.Body.Close() }()

	var got sourceVolumeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.NotNil(t, got.Sources)
	require.Empty(t, got.Sources)
}

func TestHandleChartTopSources_ComputeError_Returns500NotPanic(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)
	repo.computeErr = errTest

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/quellen")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleChartDisposition_ReturnsAllFourInFixedOrderWithDrilldownURLs(t *testing.T) {
	repo := &fakeRepository{stats: analysis.Statistics{
		VolumeByDisposition: map[report.Disposition]int{
			report.DispositionNone:       80,
			report.DispositionQuarantine: 15,
			report.DispositionReject:     5,
		},
	}}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/disposition")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got dispositionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Slices, 4)

	require.Equal(t, "none", got.Slices[0].Disposition)
	require.Equal(t, "No action", got.Slices[0].Label)
	require.Equal(t, 80, got.Slices[0].Total)
	require.Contains(t, got.Slices[0].URL, "disposition=none")

	require.Equal(t, "quarantine", got.Slices[1].Disposition)
	require.Equal(t, 15, got.Slices[1].Total)

	require.Equal(t, "reject", got.Slices[2].Disposition)
	require.Equal(t, 5, got.Slices[2].Total)

	require.Equal(t, "unknown", got.Slices[3].Disposition)
	require.Equal(t, 0, got.Slices[3].Total)
}

func TestHandleChartDisposition_ComputeError_Returns500NotPanic(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)
	repo.computeErr = errTest

	resp := httpGet(t, client, "http://"+srv.Addr()+"/api/diagramme/disposition")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
