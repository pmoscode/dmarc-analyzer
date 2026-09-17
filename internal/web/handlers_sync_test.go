package web

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/app/syncjob"
	"github.com/pmoscode/dmarc-analyzer/internal/infra/keyring"
)

func TestHandleSyncStart_StartsJobAndRedirectsBack(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+srv.Addr()+"/abgleich", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://"+srv.Addr())
	req.Header.Set("Referer", "http://"+srv.Addr()+"/berichte")
	req.Header.Set("X-CSRF-Token", srv.auth.csrfToken)

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

	resp1 := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(srv, nil))
	require.NoError(t, resp1.Body.Close())
	require.Eventually(t, func() bool { return fd.syncer.callCount() > 0 }, time.Second, 5*time.Millisecond)

	resp2 := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(srv, nil))
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	srv.deps.SyncJob.Cancel()
}

func TestHandleSyncCancel_StopsRunningJob(t *testing.T) {
	srv, fd := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	fd.syncer.block = true
	client := authenticatedClient(t, srv)

	resp := postForm(t, client, srv.Addr(), "/abgleich", csrfFormValues(srv, nil))
	require.NoError(t, resp.Body.Close())
	require.Eventually(t, func() bool { return srv.deps.SyncJob.Snapshot().Status == syncjob.StatusRunning }, time.Second, 5*time.Millisecond)

	resp = postForm(t, client, srv.Addr(), "/abgleich/abbrechen", csrfFormValues(srv, nil))
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool { return srv.deps.SyncJob.Snapshot().Status == syncjob.StatusCancelled }, time.Second, 5*time.Millisecond)
}

func TestHandleEvents_SendsCurrentStateImmediately(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+srv.Addr()+"/ereignisse", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	buf := make([]byte, 512)
	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	require.Contains(t, string(buf[:n]), `"status":"idle"`)
}

func TestHandleSyncStart_CredentialsLocked_RedirectsToUnlock(t *testing.T) {
	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	// Ersetzt das Fake durch einen echten, gesperrten LockableFileStore,
	// um credentialsLocked() über die echte Locked()-Schnittstelle
	// auszulösen (das einfache fakeCredentialStore kennt keinen
	// Sperrzustand).
	srv.deps.Credentials = keyring.NewLockableFileStore(t.TempDir())
	client := authenticatedClient(t, srv)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+srv.Addr()+"/abgleich", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://"+srv.Addr())
	req.Header.Set("X-CSRF-Token", srv.auth.csrfToken)

	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Location"), "/entsperren")
}
