package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildInfo_ShortCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		commit string
		want   string
	}{
		{name: "leer", commit: "", want: ""},
		{name: "voller hash", commit: "9a79aa6f0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f", want: "9a79aa6"},
		{name: "kurzer hash bleibt", commit: "9a79", want: "9a79"},
		{name: "dirty-suffix bleibt", commit: "9a79aa6f0c1d2e3f-dirty", want: "9a79aa6-dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, BuildInfo{Commit: tt.commit}.ShortCommit())
		})
	}
}

func TestLayout_HeaderShowsVersionAndCommit(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServerWithAccounts(t, mustAccount(t, "Konto 1"))
	client := authenticatedClient(t, srv)

	for _, path := range []string{"/", "/berichte", "/einstellungen"} {
		resp := httpGet(t, client, "http://"+srv.Addr()+path)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()

		page := string(body)
		require.Contains(t, page, `<span class="app-version">`+testBuild.Version+`</span>`, path)
		require.Contains(t, page, `</span> · <span class="app-commit" title="Commit `+testBuild.Commit+`">`+testBuild.ShortCommit()+`</span>`, path)
	}
}
