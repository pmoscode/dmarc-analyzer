package web

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
)

func TestHandleSyncStart_StartsJobAndRedirectsBack(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	req := newRequest(t, http.MethodPost, "http://"+srv.Addr()+"/abgleich", nil)
	req.Header.Set("Origin", "http://"+testPublicHost)
	req.Header.Set("Referer", "http://"+testPublicHost+"/berichte")
	req.Header.Set("X-CSRF-Token", csrfTokenFor(t, srv, client))

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Eventually(t, func() bool { return fd.syncer.callCount() > 0 }, time.Second, 5*time.Millisecond)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Redirect landet auf /berichte (Referer), Client folgt ihm.
	require.Contains(t, string(body), "Berichte")
}

func TestHandleSyncStart_AlreadyRunning_StillRedirectsWithoutError(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	fd.syncer.block = true
	client := authenticatedClient(t, srv)

	resp1 := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(t, srv, client))
	require.NoError(t, resp1.Body.Close())
	require.Eventually(t, func() bool { return fd.syncer.callCount() > 0 }, time.Second, 5*time.Millisecond)

	resp2 := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(t, srv, client))
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	srv.deps.SyncJob.Cancel()
}

func TestHandleSyncCancel_StopsRunningJob(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	fd.syncer.block = true
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(t, srv, client))
	require.NoError(t, resp.Body.Close())
	require.Eventually(t, func() bool { return srv.deps.SyncJob.Snapshot().Status == syncjob.StatusRunning }, time.Second, 5*time.Millisecond)

	resp = postForm(t, client, srv.Addr(), "/abgleich/abbrechen", csrfFormValues(t, srv, client))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool { return srv.deps.SyncJob.Snapshot().Status == syncjob.StatusCancelled }, time.Second, 5*time.Millisecond)
}

func TestHandleEvents_SendsCurrentStateImmediately(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	resp, err := client.Do(newRequest(t, http.MethodGet, "http://"+srv.Addr()+"/ereignisse", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	buf := make([]byte, 512)
	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	require.Contains(t, string(buf[:n]), `"status":"idle"`)
}
