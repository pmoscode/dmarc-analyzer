package paths_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pmoscode/dmarc-analyzer/internal/platform/paths"
)

// isolateHome zeigt alle von os.UserConfigDir()/os.UserHomeDir() gelesenen
// Umgebungsvariablen auf ein temporäres Verzeichnis, damit Tests nicht in
// das echte Home-Verzeichnis des CI-Runners schreiben — unabhängig davon,
// ob dieser unter Linux, macOS oder Windows läuft.
func isolateHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("APPDATA", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
}

func TestConfigDir_ContainsAppName(t *testing.T) {
	isolateHome(t)

	dir, err := paths.ConfigDir()
	require.NoError(t, err)
	require.Contains(t, dir, "dmarc-analyzer")

	info, err := os.Stat(dir)
	require.NoError(t, err, "Verzeichnis wurde nicht angelegt")
	require.True(t, info.IsDir())
}

func TestDatabasePath_EndsWithDBFile(t *testing.T) {
	isolateHome(t)

	dbPath, err := paths.DatabasePath()
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(dbPath, "dmarc.db"))
}

func TestLogDir_IsCreated(t *testing.T) {
	isolateHome(t)

	dir, err := paths.LogDir()
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err, "Verzeichnis wurde nicht angelegt")
	require.True(t, info.IsDir())
}
