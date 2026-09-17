package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/statistics"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/analysis"
	"github.com/pmoscode/dmarc-analyzer/internal/domain/report"
)

// authenticatedClient meldet sich einmalig über einen frischen Einmal-Code
// an und liefert einen Client, dessen Cookie-Jar die Sitzung für alle
// folgenden Anfragen an srv mitträgt.
func authenticatedClient(t *testing.T, srv *Server) *http.Client {
	t.Helper()

	code, err := srv.auth.issueCode()
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	resp := httpGet(t, client, "http://"+srv.Addr()+"/anmelden?code="+code)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return client
}

func newTestServerWithRepo(t *testing.T, repo *fakeRepository) *Server {
	t.Helper()
	isolateConfigDir(t)

	deps := Dependencies{Statistics: &statistics.UseCase{Repository: repo}}
	srv, err := New(deps, Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	_, err = srv.Start(context.Background())
	require.NoError(t, err)
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
		"zwei Quellen mit demselben erkannten Dienst dürfen im Diagramm nicht in derselben Zeile landen")
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
	// Der Fehler wird erst nach der Anmeldung gesetzt: Die Anmeldung
	// selbst leitet auf "/" weiter und der Test-Client folgt dem
	// Redirect — die Übersicht ruft ebenfalls Statistics.Dashboard auf
	// und würde mit computeErr von Anfang an schon beim Anmelden
	// scheitern, nicht erst bei der hier zu prüfenden Anfrage.
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
