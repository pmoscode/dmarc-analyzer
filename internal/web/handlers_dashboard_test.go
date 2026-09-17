package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandleDashboard_RendersFilterBarWithSelectedValues(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/?zeitraum=90&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.Contains(t, html, `value="90" selected`)
	require.Contains(t, html, `value="example.com"`)
}

func TestHandleDashboard_FilterParams_ReachRepository(t *testing.T) {
	repo := &fakeRepository{}
	srv := newTestServerWithRepo(t, repo)
	client := authenticatedClient(t, srv)

	resp := httpGet(t, client, "http://"+srv.Addr()+"/?zeitraum=7&domain=example.com")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "example.com", repo.lastDailyVolumesQuery.Domain)
}
