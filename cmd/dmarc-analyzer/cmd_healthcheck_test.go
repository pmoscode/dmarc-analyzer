package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// listenerPort extrahiert den Port aus der Adresse eines net.Listener —
// httptest.NewServer bindet auf einen zufälligen Port, den der Test dann
// über DMARC_LISTEN_ADDR bekannt geben muss.
func listenerPort(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	return port
}

func TestRunHealthcheck_ServerHealthy_ReturnsNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/gesund", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("DMARC_LISTEN_ADDR", "127.0.0.1:"+listenerPort(t, srv.Listener.Addr().String()))

	require.NoError(t, runHealthcheck(context.Background(), nil))
}

func TestRunHealthcheck_ServerReturnsError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv("DMARC_LISTEN_ADDR", "127.0.0.1:"+listenerPort(t, srv.Listener.Addr().String()))

	require.Error(t, runHealthcheck(context.Background(), nil))
}

func TestRunHealthcheck_ServerUnreachable_ReturnsError(t *testing.T) {
	t.Setenv("DMARC_LISTEN_ADDR", "127.0.0.1:1")

	require.Error(t, runHealthcheck(context.Background(), nil))
}

func TestRunHealthcheck_MalformedListenAddr_FallsBackToDefaultPort(t *testing.T) {
	t.Setenv("DMARC_LISTEN_ADDR", "kein-gueltiges-addr-format")

	err := runHealthcheck(context.Background(), nil)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "healthcheck-anfrage konnte nicht gebaut werden",
		"ein ungültiges DMARC_LISTEN_ADDR sollte auf den Standardport zurückfallen, nicht die Anfrage kaputt machen")
}
