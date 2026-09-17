package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindRunningInstance_NoFile_ReturnsFalse(t *testing.T) {
	isolateConfigDir(t)

	_, ok := FindRunningInstance()
	require.False(t, ok)
}

func TestFindRunningInstance_MalformedFile_ReturnsFalse(t *testing.T) {
	isolateConfigDir(t)

	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	appDir := filepath.Join(dir, "dmarc-analyzer")
	require.NoError(t, os.MkdirAll(appDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "instance.json"), []byte("{kaputt"), 0o600))

	_, ok := FindRunningInstance()
	require.False(t, ok)
}

func TestFindRunningInstance_IncompleteFile_ReturnsFalse(t *testing.T) {
	isolateConfigDir(t)

	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	appDir := filepath.Join(dir, "dmarc-analyzer")
	require.NoError(t, os.MkdirAll(appDir, 0o700))
	data, err := json.Marshal(instanceFile{Port: 0, PID: 123, Secret: ""})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "instance.json"), data, 0o600))

	_, ok := FindRunningInstance()
	require.False(t, ok)
}

func TestFindRunningInstance_ValidFile_ReturnsPortAndSecret(t *testing.T) {
	srv := newTestServer(t)

	inst, ok := FindRunningInstance()
	require.True(t, ok)
	require.Equal(t, srv.auth.instanceSecret, inst.Secret)
	require.Greater(t, inst.Port, 0)
}

func TestRequestLoginURL_RunningInstance_GrantsWorkingSession(t *testing.T) {
	newTestServer(t)

	inst, ok := FindRunningInstance()
	require.True(t, ok)

	loginURL, err := RequestLoginURL(context.Background(), inst)
	require.NoError(t, err)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}
	resp := httpGet(t, client, loginURL)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequestLoginURL_WrongSecret_Fails(t *testing.T) {
	newTestServer(t)

	inst, ok := FindRunningInstance()
	require.True(t, ok)
	inst.Secret = "definitiv-falsches-geheimnis"

	_, err := RequestLoginURL(context.Background(), inst)
	require.Error(t, err)
}

func TestRequestLoginURL_NoInstanceListening_Fails(t *testing.T) {
	_, err := RequestLoginURL(context.Background(), RunningInstance{Port: 1, Secret: "irrelevant"})
	require.Error(t, err)
}

func TestHandleInternalCode_WrongSecret_Returns401(t *testing.T) {
	srv := newTestServer(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://"+srv.Addr()+"/intern/code", strings.NewReader("falsch"))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
